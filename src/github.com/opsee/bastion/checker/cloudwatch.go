package checker

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/opsee/basic/schema"
	"github.com/opsee/bastion/config"
	opsee_types "github.com/opsee/protobuf/opseeproto/types"
)

const cloudwatchWorkerTaskType = "CloudWatchRequest"

var (
	cloudwatchClient           *cloudwatch.CloudWatch
	CloudWatchStatisticsPeriod = 60
)

func init() {
	cfg := config.GetConfig()

	sess, err := cfg.AWS.Session()
	if err != nil {
		log.WithError(err).Fatal("Couldn't get aws session from global config")
	}

	cloudwatchClient = cloudwatch.New(sess)
	Recruiters.RegisterWorker(cloudwatchWorkerTaskType, NewCloudWatchWorker)
}

type CloudWatchRequest struct {
	Target                 *schema.Target
	Metrics                []*schema.CloudWatchMetric
	Namespace              string
	StatisticsIntervalSecs int
	StatisticsPeriod       int
	Statistics             []*string
}

type MetricStatisticsResponse struct {
	Index  int
	Error  error
	Metric *schema.Metric
}

// TODO(dan) should we add a target to the metric or assume that they all have the same target
func (this *CloudWatchRequest) GetDimensions(metric *schema.CloudWatchMetric) ([]*cloudwatch.Dimension, error) {
	switch metric.Namespace {
	case "AWS/RDS":
		return []*cloudwatch.Dimension{
			&cloudwatch.Dimension{
				Name:  aws.String("DBInstanceIdentifier"),
				Value: aws.String(this.Target.Id),
			},
		}, nil
	case "AWS/EC2":
		return []*cloudwatch.Dimension{
			&cloudwatch.Dimension{
				Name:  aws.String("InstanceId"),
				Value: aws.String(this.Target.Id),
			},
		}, nil
	default:
		return nil, fmt.Errorf("Couldn't get dimensions for %T namespace %s", this, metric.Namespace)
	}
}

func (this *CloudWatchRequest) Do() <-chan *Response {
	respChan := make(chan *Response, 1)
	responseMetrics := []*schema.Metric{}
	responseErrors := []*opsee_types.Error{}

	for _, metric := range this.Metrics {
		// 1 minute lag.  otherwise we won't get stats
		endTime := time.Now().UTC().Add(time.Duration(-1) * time.Minute)
		startTime := endTime.Add(time.Duration(-1*this.StatisticsIntervalSecs) * time.Second)
		log.WithFields(log.Fields{"startTime": startTime, "endTime": endTime}).Debug("Fetching cloudwatch metric statistics")

		dimensions, err := this.GetDimensions(metric)
		if err != nil {
			log.WithError(err).Error("Couldn't get dimensions")
			responseErrors = append(responseErrors, opsee_types.NewError(metric.Name, err.Error()))

			// TODO(dan) add error to CloudWatchResponse
			continue
		}

		params := &cloudwatch.GetMetricStatisticsInput{
			StartTime:  aws.Time(startTime),
			EndTime:    aws.Time(endTime),
			MetricName: aws.String(metric.Name),
			Namespace:  aws.String(metric.Namespace),
			Period:     aws.Int64(int64(this.StatisticsPeriod)),
			Statistics: this.Statistics,
			Dimensions: dimensions,
		}

		// output for aws cli so you can validate results to this call
		if dimensions != nil && len(dimensions) > 0 {
			log.Debugf("aws cloudwatch get-metric-statistics --metric-name %s --start-time %s --end-time %s --period %d --namespace %s --statistics Average --dimensions Name=%s,Value=%s", metric.Name, startTime.Format("2006-01-02T15:04:05"), endTime.Format("2006-01-02T15:04:05"), this.StatisticsPeriod, metric.Namespace, *dimensions[0].Name, *dimensions[0].Value)
		}

		resp, err := cloudwatchClient.GetMetricStatistics(params)
		if err != nil {
			// TODO(dan) add error to CloudWatchResponse
			log.WithError(err).Errorf("Couldn't get metric statistics for %s", metric.Name)
			continue
		}

		if len(resp.Datapoints) == 0 {
			// TODO(dan) add error to CloudWatchResponse
			log.WithError(err).Errorf("No datapoints for %s", metric.Name)
			continue
		}

		// wrap datapoints in schema.Metric and append to slice of all Metrics
		// NOTE(dan) we're only using one datapoint at the moment
		// TODO(dan) datapoint[0] should be most recent
		for _, datapoint := range resp.Datapoints {
			for _, statistic := range this.Statistics {
				value := float64(0.0)
				switch aws.StringValue(statistic) {
				case "Average":
					value = aws.Float64Value(datapoint.Average)
				case "Maximum":
					value = aws.Float64Value(datapoint.Maximum)
				case "Minimum":
					value = aws.Float64Value(datapoint.Minimum)
				case "SampleCount":
					value = aws.Float64Value(datapoint.SampleCount)
				case "Sum":
					value = aws.Float64Value(datapoint.Sum)
				default:
					log.Errorf("Unknown statistic type %s", statistic)
				}

				timestamp := &opsee_types.Timestamp{}
				timestamp.Scan(aws.TimeValue(datapoint.Timestamp))
				metric := &schema.Metric{
					Name:      metric.Name,
					Value:     value,
					Timestamp: timestamp,
					Unit:      aws.StringValue(datapoint.Unit),
					Statistic: aws.StringValue(statistic),
				}
				responseMetrics = append(responseMetrics, metric)
				log.WithFields(log.Fields{
					"Name":      metric.Name,
					"Value":     value,
					"Timestamp": timestamp,
					"Unit":      aws.StringValue(datapoint.Unit),
					"Statistic": aws.StringValue(statistic)}).Debug("received datapoint")
			}
			break
		}
	}

	cloudwatchResponse := &schema.CloudWatchResponse{
		Namespace: this.Namespace,
		Metrics:   responseMetrics,
		Errors:    responseErrors,
	}

	respChan <- &Response{
		Response: cloudwatchResponse,
	}

	return respChan
}

type CloudWatchWorker struct {
	workerQueue chan Worker
}

func NewCloudWatchWorker(queue chan Worker) Worker {
	return &CloudWatchWorker{
		workerQueue: queue,
	}
}

func (this *CloudWatchWorker) Work(ctx context.Context, task *Task) *Task {
	defer func() {
		this.workerQueue <- this
	}()

	if ctx.Err() != nil {
		task.Response = &Response{
			Error: ctx.Err(),
		}
		return task
	}

	request, ok := task.Request.(*CloudWatchRequest)
	if ok {
		log.Debugf("Cloudwatch request: %v", request)
		select {
		case response := <-request.Do():
			if response.Error != nil {
				log.WithError(response.Error).Errorf("error processing request: %s", *task)
			}
			task.Response = response
		case <-ctx.Done():
			task.Response = &Response{
				Error: ctx.Err(),
			}
		}
	} else {
		task.Response = &Response{
			Error: fmt.Errorf("Unable to process request: %s", task.Request),
		}
	}

	log.Debug("response: ", task.Response)
	return task
}
