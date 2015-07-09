package messaging

type Adapter struct {
	In       <-chan interface{}
	Out      chan<- string
	consumer *Consumer
	producer *Producer
}

func NewAdapter(consumerTopic string, consumerChannel string, producerTopic string) *Adapter {
	return nil
}
