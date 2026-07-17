package domain

import "context"

type Message struct {
	Topic    string `json:"topic"`
	Sequence uint64 `json:"sequence"`
	Payload  []byte `json:"payload"`
}
type Broker interface {
	Publish(context.Context, Message) error
	Subscribe(context.Context, string) (<-chan Message, error)
}
