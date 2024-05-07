package signalflow

import "errors"

var (
	ErrNilChannel                = errors.New("RabbitMQ channel is nil")
	ErrServerBlocked             = errors.New("RabbitMQ server is blocked")
	ErrConnectionClosed          = errors.New("RabbitMQ connection is closed")
	ErrUnsupportedOptionArgument = errors.New("unsupported option argument")
	ErrAckFailure                = errors.New("sending ack to server failed while the message already processed.")
)
