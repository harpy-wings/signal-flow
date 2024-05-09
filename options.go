package signalflow

import "github.com/streadway/amqp"

type Option func(*signalFlow[any]) error

// OptionQueueDeclaration declares a queue in the RMQ server.
func OptionQueueDeclaration(req QueueDeclarationRequest) Option {
	return func(sf *signalFlow[any]) error {
		// add validation check here.
		sf.config.onConnectionStabilized = append(sf.config.onConnectionStabilized, func(channel *amqp.Channel) error {
			Queue, err := channel.QueueDeclare(req.Name, req.Durable, req.AutoDelete, req.Elusive, req.NoWait, req.Args)
			if err != nil {
				return err
			}
			sf.logger.Infof("Queue %s declared.", Queue.Name)
			return nil
		})
		return nil
	}
}

// OptionExchangeDeclaration declares an exchange in the RMQ server.
func OptionExchangeDeclaration(req ExchangeDeclarationRequest) Option {
	return func(sf *signalFlow[any]) error {
		// add validation check here.
		sf.config.onConnectionStabilized = append(sf.config.onConnectionStabilized, func(channel *amqp.Channel) error {
			err := channel.ExchangeDeclare(req.Name, req.Kind, req.Durable, req.AutoDelete, req.Internal, req.NoWait, req.Args)
			if err != nil {
				return err
			}
			sf.logger.Infof("Exchange %s(%s) declared.", req.Name, req.Kind)
			return nil
		})

		return nil
	}
}

// OptionBinding binds an exchange to another exchange or queue in the RMQ server.
// NOTE: Ensure that the queue is declared before binding.
// For example, `New(OptionBinding,OptionQueueDeclaration,OptionExchangeDeclaration)` fails while `New(OptionQueueDeclaration,OptionExchangeDeclaration,OptionBinding)` will succeed.
func OptionBinding(req BindingRequest) Option {
	return func(sf *signalFlow[any]) error {
		// add validation check here.
		switch req.DestinationType {
		case BindingDestinationTypeQueue:
			sf.config.onConnectionStabilized = append(sf.config.onConnectionStabilized, func(channel *amqp.Channel) error {
				err := channel.QueueBind(req.Destination, req.RoutingKey, req.Source, req.NoWait, req.Args)
				if err != nil {
					return err
				}
				sf.logger.Infof("Binding %s--[%s]-->%s | noWait(%t)", req.Source, req.RoutingKey, req.Destination, req.NoWait)
				return nil
			})
		case BindingDestinationTypeExchange:
			sf.config.onConnectionStabilized = append(sf.config.onConnectionStabilized, func(channel *amqp.Channel) error {
				err := channel.ExchangeBind(req.Destination, req.RoutingKey, req.Source, req.NoWait, req.Args)
				if err != nil {
					return err
				}
				sf.logger.Infof("Binding %s--[%s]-->%s | noWait(%t)", req.Source, req.RoutingKey, req.Destination, req.NoWait)
				return nil
			})
		default:
			return ErrUnsupportedOptionArgument
		}

		return nil
	}
}

// OptionWithQoS sets the Quality of Service (QoS) for the RMQ server.
func OptionWithGlobalQoS(prefetchCount int, prefetchSize int) Option {
	return func(sf *signalFlow[any]) error {

		sf.config.onConnectionStabilized = append(sf.config.onConnectionStabilized, func(channel *amqp.Channel) error {
			err := channel.Qos(prefetchCount, prefetchSize, true)
			if err != nil {
				return err
			}
			return nil
		})

		return nil
	}
}

// OptionWithQueueName sets the queue name for the signal flow client. If the queue name exists, the signal flow creates a consumer for the queue.
// The ForeachN function will be callable.
func OptionWithQueueName(v string) Option {
	return func(sf *signalFlow[any]) error {
		sf.config.queueName = v
		return nil
	}
}

// OptionWithExchangeName sets the exchange name for the signal flow client. If the exchange name exists, the signal flow creates a producer for the exchange.
// The Emit function will be callable.
func OptionWithExchangeName(v string) Option {
	return func(sf *signalFlow[any]) error {
		sf.config.exchangeName = v
		return nil
	}
}

// OptionWithHostname sets the host URL of the Signal flow.
// Example: OptionWithHost("amqp://guest:guest@localhost:5672/")
func OptionWithHost(v string) Option {
	return func(sf *signalFlow[any]) error {
		sf.config.host = v
		return nil
	}
}

// OptionWithName sets the name of the Signal flow Client. The name will be used as the consumer name and also for logging purposes.
func OptionWithName(v string) Option {
	return func(sf *signalFlow[any]) error {
		sf.config.name = v
		return nil
	}
}

// OptionWithCodec specifies the Codec for encoding and decoding messages.
// Example: OptionWithCodec(codec.NewJsonCodec())
func OptionWithCodec(c Codec) Option {
	return func(sf *signalFlow[any]) error {
		sf.codec = c
		return nil
	}
}

// OptionWithRoutingKey specifies the routing key for the Producer once it emits messages.
func OptionWithRoutingKey(v string) Option {
	return func(sf *signalFlow[any]) error {
		sf.config.routingKey = v
		return nil
	}
}

func OptionWithErrorHandler(fn func(error)) Option {
	return func(sf *signalFlow[any]) error {
		sf.config.errorHandler = fn
		return nil
	}
}
