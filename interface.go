package signalflow

type SignalFlow[Message any] interface {

	// ForeachN implements a bounded parallelism pattern, executing the `fn` function for each message in `n` goroutines. It acknowledges the RMQ only if the `fn` returns nil.
	ForeachN(fn func(Message) error, n int) error

	// Emit sends the message to all registered exchanges. If it fails to send the message, it returns an error; otherwise, it returns nil.
	Emit(Message) error
}
