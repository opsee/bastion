package bastion

import (
	"fmt"

	"github.com/opsee/bastion/checker"
	"github.com/opsee/bastion/messaging"
)

func main() {
	request := &checker.HTTPRequest{
		Method: "GET",
		URL:    "http://api-beta.opsee.co/health_check",
	}

	producer, err := messaging.NewProducer("checks")
	if err != nil {
		panic(err)
	}
	consumer, _ := messaging.NewConsumer("results", "testing")

	fmt.Println("Publishing event: ", request)
	producer.Publish(request)

	fmt.Println("Waiting for event...")
	event := <-consumer.Channel()
	fmt.Println("Received event: ", event)
	event.Ack()
}
