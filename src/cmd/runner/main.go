package main

import (
	"fmt"
	"golang.org/x/net/context"
	"os"
	"os/signal"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nsqio/go-nsq"
	"github.com/opsee/bastion/checker"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/logging"
)

const (
	moduleName          = "runner"
	maxConcurrentChecks = 10
)

var (
	logger = logging.GetLogger(moduleName)
)

func main() {
	var err error

	config := config.GetConfig()
	nsqdHost := os.Getenv("NSQD_HOST")
	customerID := os.Getenv("CUSTOMER_ID")

	logger.Info("Starting %s...", moduleName)
	// XXX: Holy fuck make logging easier.
	logging.SetLevel(config.LogLevel, moduleName)
	logging.SetLevel(config.LogLevel, "messaging")
	logging.SetLevel(config.LogLevel, "scanner")

	runner := checker.NewRunner(checker.NewResolver(config))

	nsqConfig := nsq.NewConfig()
	consumer, err := nsq.NewConsumer("checks", "runner", nsqConfig)
	if err != nil {
		logger.Fatal(err.Error())
	}

	if err := consumer.ConnectToNSQD(nsqdHost); err != nil {
		logger.Fatal(err.Error())
	}

	producer, err := nsq.NewProducer(nsqdHost, nsqConfig)
	if err != nil {
		logger.Fatal(err.Error())
	}

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		logger.Fatal(err.Error())
	}

	consumer.AddConcurrentHandlers(nsq.HandlerFunc(func(m *nsq.Message) error {
		// Message is a Check
		// We emit a CheckResult
		check := &checker.Check{}
		if err := proto.Unmarshal(m.Body, check); err != nil {
			return err
		}

		d, err := time.ParseDuration(fmt.Sprintf("%ds", check.Interval))
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(d*2))
		responseChan, err := runner.RunCheck(ctx, check)
		if err != nil {
			cancel()
			return err
		}

		var responses []*checker.CheckResponse

		for response := range responseChan {
			responses = append(responses, response)
		}

		timestamp := &checker.Timestamp{
			Seconds: int64(time.Now().Second()),
		}
		result := &checker.CheckResult{
			CustomerId: customerID,
			CheckId:    check.Id,
			Timestamp:  timestamp,
			Responses:  responses,
		}

		msg, err := proto.Marshal(result)
		if err != nil {
			cancel()
			return err
		}

		if err := producer.Publish("results", msg); err != nil {
			cancel()
			return err
		}

		return nil
	}), maxConcurrentChecks)

	if err := consumer.ConnectToNSQD(nsqdHost); err != nil {
		logger.Fatal(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, os.Kill)

	for {
		select {
		case s := <-sigs:
			logger.Info("Received signal %s. Stopping...", s)
			consumer.Stop()
			<-consumer.StopChan
			producer.Stop()
			os.Exit(0)
		case err := <-heart.Beat():
			logger.Error(err.Error())
		}
	}

}
