package signalflow

import (
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

type config struct {
	host                   string                 // RMQ host.
	name                   string                 // Name of the consumer. By default, a random number with "sg_" prefix.
	queueName              string                 // Queue name for the consumer to consume messages.
	exchangeName           string                 // Exchange name where the producer will push messages.
	routingKey             string                 // Routing key for the producer to emit messages into the exchange.
	exclusive              bool                   // Whether the consumer should be exclusive or not. Default is false.
	flowControlBufferSize  int                    // Buffer size of channel for flow control purposes. Default is 1.
	amqAttemptsLimit       int                    // Attempts limit for retrying.
	priority               uint8                  // Default is 0; range is 0 to 9.
	mandatory              bool                   // Default is false.
	immediate              bool                   // Default is false.
	args                   map[string]interface{} // Default arguments passed to RMQ server in every request.
	errorHandler           func(error)            // Function called to handle asynchronous errors.
	onConnectionStabilized []func(channel *amqp.Channel) error
	logger                 logrus.FieldLogger
	codec                  Codec

	// ackTimeout                time.Duration
	// numberOfGoRoutinesForeach int
}
