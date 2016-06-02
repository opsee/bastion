package checker

import (
	"crypto/tls"
	"fmt"
	"sort"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/opsee/basic/schema"
	opsee_aws_cloudwatch "github.com/opsee/basic/schema/aws/cloudwatch"
	opsee "github.com/opsee/basic/service"
	"github.com/opsee/bastion/config"
	opsee_types "github.com/opsee/protobuf/opseeproto/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	cloudwatchWorkerTaskType = "CloudWatchRequest"
	datapointWindowRewind    = 10
)

var (
	BezosClient                opsee.BezosClient
	CloudWatchStatisticsPeriod = 60
)

type metricList []*schema.Metric

func (l metricList) Len() int           { return len(l) }
func (l metricList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l metricList) Less(i, j int) bool { return l[i].Timestamp.Millis() < l[j].Timestamp.Millis() }

func init() {
	Recruiters.RegisterWorker(cloudwatchWorkerTaskType, NewCloudWatchWorker)
}

func ConnectCloudwatchBezosClient() error {
	bezosConn, err := grpc.Dial(
		config.GetConfig().BezosHost,
		grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{
				InsecureSkipVerify: false,
			}),
		),
	)

	if err != nil {
		return err
	}

	BezosClient = opsee.NewBezosClient(bezosConn)

	return nil
}

type CloudWatchRequest struct {
	User                   *schema.User
	Region                 string
	VpcId                  string
	MaxAge                 time.Duration
	Target                 *schema.Target
	Metrics                []*schema.CloudWatchMetric
	Namespace              string
	StatisticsIntervalSecs int
	StatisticsPeriod       int
	Statistics             []string
}

type MetricStatisticsResponse struct {
	Index  int
	Error  error
	Metric *schema.Metric
}

// TODO(dan) should we add a target to the metric or assume that they all have the same target
func (this *CloudWatchRequest) GetDimensions(metric *schema.CloudWatchMetric) ([]*opsee_aws_cloudwatch.Dimension, error) {
	switch metric.Namespace {
	case "AWS/RDS":
		return []*opsee_aws_cloudwatch.Dimension{
			&opsee_aws_cloudwatch.Dimension{
				Name:  aws.String("DBInstanceIdentifier"),
				Value: aws.String(this.Target.Id),
			},
		}, nil
	case "AWS/EC2":
		return []*opsee_aws_cloudwatch.Dimension{
			&opsee_aws_cloudwatch.Dimension{
				Name:  aws.String("InstanceId"),
				Value: aws.String(this.Target.Id),
			},
		}, nil
	case "AWS/AutoScaling":
		return []*opsee_aws_cloudwatch.Dimension{
			&opsee_aws_cloudwatch.Dimension{
				Name:  aws.String("AutoScalingGroupName"),
				Value: aws.String(this.Target.Id),
			},
		}, nil
	default:
		return nil, fmt.Errorf("Couldn't get dimensions for %T namespace %s", this, metric.Namespace)
	}
}

func (this *CloudWatchRequest) Do(ctx context.Context) <-chan *Response {
	respChan := make(chan *Response, 1)
	responseMetrics := []*schema.Metric{}
	responseErrors := []*opsee_types.Error{}

	for _, metric := range this.Metrics {
		// 1 minute lag.  otherwise we won't get stats
		endTs := &opsee_types.Timestamp{}
		startTs := &opsee_types.Timestamp{}
		endTime := time.Now().UTC().Add(time.Duration(-1) * time.Minute)
		startTime := endTime.Add(time.Duration(-datapointWindowRewind*this.StatisticsIntervalSecs) * time.Second)
		endTs.Scan(endTime)
		startTs.Scan(startTime)
		log.WithFields(log.Fields{"startTime": startTime, "endTime": endTime}).Debug("Fetching cloudwatch metric statistics")

		dimensions, err := this.GetDimensions(metric)
		if err != nil {
			log.WithError(err).Error("Couldn't get dimensions")
			responseErrors = append(responseErrors, opsee_types.NewError(metric.Name, err.Error()))

			// TODO(dan) add error to CloudWatchResponse
			continue
		}

		params := &opsee_aws_cloudwatch.GetMetricStatisticsInput{
			StartTime:  startTs,
			EndTime:    endTs,
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

		maxAge := &opsee_types.Timestamp{}
		maxAge.Scan(time.Now().UTC().Add(this.MaxAge * -2))

		resp, err := BezosClient.Get(
			ctx,
			&opsee.BezosRequest{
				User:   this.User,
				Region: this.Region,
				VpcId:  this.VpcId,
				MaxAge: maxAge,
				Input:  &opsee.BezosRequest_Cloudwatch_GetMetricStatisticsInput{params},
			})
		if err != nil {
			// TODO(dan) add error to CloudWatchResponse
			log.WithError(err).Errorf("Couldn't get metric statistics for %s", metric.Name)
			continue
		}
		output := resp.GetCloudwatch_GetMetricStatisticsOutput()
		if output == nil {
			log.WithError(err).Errorf("error decoding aws response")
			continue
		}

		if len(output.Datapoints) == 0 {
			// TODO(dan) add error to CloudWatchResponse
			log.WithError(err).Errorf("No datapoints for %s", metric.Name)
			continue
		}

		// wrap datapoints in schema.Metric and append to slice of all Metrics
		// NOTE(dan) we're only using one datapoint at the moment
		// TODO(dan) datapoint[0] should be most recent
		for _, datapoint := range output.Datapoints {
			for _, statistic := range this.Statistics {
				value := float64(0.0)
				switch statistic {
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
				timestamp.Scan(datapoint.Timestamp)
				metric := &schema.Metric{
					Name:      metric.Name,
					Value:     value,
					Timestamp: timestamp,
					Unit:      *datapoint.Unit,
					Statistic: statistic,
				}
				responseMetrics = append(responseMetrics, metric)
				log.WithFields(log.Fields{
					"Name":      metric.Name,
					"Value":     value,
					"Timestamp": timestamp,
					"Unit":      datapoint.Unit,
					"Statistic": statistic}).Debug("received datapoint")
			}
			break
		}
	}

	sort.Sort(metricList(responseMetrics))

	cloudwatchResponse := &schema.CloudWatchResponse{
		Namespace: this.Namespace,
		Metrics:   responseMetrics,
		Errors:    responseErrors,
	}

	respChan <- &Response{
		Response: &schema.CheckResponse_CloudwatchResponse{cloudwatchResponse},
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
		case response := <-request.Do(ctx):
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
