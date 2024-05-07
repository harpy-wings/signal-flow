package signalflow

// SignalFlow is an interface for handling AMQP operations, including retrying and reconnecting strategies. It facilitates message emission and retrieval from AMQP brokers,
// incorporating flow control signals. It is designed generically to support various data formats.
type SignalFlow[Message any] interface {

	// ForeachN implements a bounded parallelism pattern, executing the `fn` function for each message.
	// It acknowledges the RMQ only if the `fn` returns nil.
	Foreach(fn func(Message) error) error

	// Emit sends the message to all registered exchanges, utilizing the default routing key.
	// If the emission fails, it returns an error; otherwise, it returns nil.
	// See OptionWithRoutingKey for setting default routing keys.
	Emit(msg Message) error

	// EmitR sends the message to all registered exchanges, using the provided routing key.
	// If the emission fails, it returns an error; otherwise, it returns nil.
	EmitR(msg Message, routingKey string) error
}
