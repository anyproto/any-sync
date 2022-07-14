package sync

type PubSubPayload struct {
}

type PubSub interface {
	Send(msg *PubSubPayload) error
	Listen(chan *PubSubPayload) error
}

func NewPubSub(topic string) PubSub {
	return nil
}
