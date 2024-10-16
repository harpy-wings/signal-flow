# signal-flow

[![Go Report Card](https://goreportcard.com/badge/github.com/harpy-wings/signal-flow)](https://goreportcard.com/report/github.com/harpy-wings/signal-flow)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=harpy-wings_signal-flow&metric=coverage)](https://sonarcloud.io/summary/new_code?id=harpy-wings_signal-flow)
[![Go Reference](https://pkg.go.dev/badge/github.com/harpy-wings/signal-flow.svg)](https://pkg.go.dev/github.com/harpy-wings/signal-flow)
[![build workflow](https://github.com/harpy-wings/signal-flow/actions/workflows/common.yml/badge.svg)](https://github.com/harpy-wings/signal-flow/actions)
[![codecov](https://codecov.io/gh/harpy-wings/signal-flow/branch/main/graph/badge.svg?token=CSO8PJZ0NU)](https://codecov.io/gh/harpy-wings/signal-flow)
[![Reliability Rating](https://sonarcloud.io/api/project_badges/measure?project=harpy-wings_signal-flow&metric=reliability_rating)](https://sonarcloud.io/summary/new_code?id=harpy-wings_signal-flow)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=harpy-wings_signal-flow&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=harpy-wings_signal-flow)

Signal-Flow is a library for handling AMQP operations, including retrying and reconnecting strategies. It facilitates message emission and retrieval from AMQP brokers, incorporating flow control signals. It is designed generically to support various data formats.

**HLD:**

```plaintext
                Queue
                ↧ 1:n (Foreach)
+―――――――――――――――――――+
| SignalFlow        | 
| Foreach           |
| Emit              | ↦ Exchange
+―――――――――――――――――――+ 1:n (Emit)


```

## Quick Start

Install Signal-Flow:

```shell
go get -u github.com/harpy-wings/signal-flow
```

Then, use it in your code:

```go
package main

import (
  "log"
  "time"

  signalflow "github.com/harpy-wings/signal-flow"
)

func main() {
  // Define a custom data structure.
  type Telegraph struct {
    Text string `json:"text"`
  }

  // Initialize Signal-Flow with necessary configurations.
  SF, err := signalflow.New[Telegraph](
    signalflow.OptionWithHost("amqp://guest:guest@localhost:5672/"), // Set AMQP server host.
    signalflow.OptionWithQueueName("qt1"),                           // Specify the queue name.
    signalflow.OptionWithExchangeName("ext1"),                       // Specify the exchange name.
  )
  if err != nil {
    panic(err)
  }

  // Consume messages from the queue.
  SF.Foreach(func(m Telegraph) error {
    log.Println(m.Text)
    return nil
  })

  // Emit a message to the exchange.
  err = SF.Emit(Telegraph{
    Text: "Hello Gopher 1!",
  })
  if err != nil {
    panic(err)
  }
}
```

See [_example](https://github.com/harpy-wings/signal-flow/tree/main/_example) for more.

## More

### Goals

The aim of this library is to create a very easy-to-use interface for AMQP brokers. Maintenance is limited to improving documentation, adding more end-to-end test functionality, fixing raised issues, and adding benchmark tests.

### Supported Go Versions

The library is developed by Go 1.22.1; however, the CI tests run on different Go versions >= 1.19. Check the Actions for more details.

### Why You Should Use Signal-Flow?

- Increase test coverage by mocking the Signal-Flow library.
- Avoid dealing with deadlocks, parallelism, and flow control of AMQP.
- No need to handle retrying the connection or recovering the channel once it disconnects.
- Reduce boilerplate code in your project.

### Interface

```go
type SignalFlow[Message any] interface {
  Foreach(fn func(Message) error) error
  Emit(msg Message) error
  EmitR(msg Message, routingKey string) error
}
```

### `Foreach(fn func(Message) error) error`

`Foreach` executes the provided `fn` function for every message received. If the `fn` function returns `nil`, the message is acknowledged (Ack); otherwise, it is not.

### `Emit(msg Message) error`

`Emit` sends a message to the preset exchange with the preset routing-key. If it fails, it returns an error.

### `EmitR(msg Message, routingKey string)`

`Emit` sends a message to the preset exchange with the given routing-key. If it fails, it returns an error.

## Best Practices

### Don't Use Singleton Pattern

Inside an application, there could be multiple modules consuming or producing different types of messages. Make sure to create separate Signal-Flow clients for each module.

### Implement Idempotence

Acknowledgment of a message is asynchronous, so you might receive the same message from the RMQ Broker multiple times due to unexpected network errors. Ensure your message processing is idempotent.

## Advanced Configuration

The following Options are available for Signal-Flow configuration:

```go
func New[Message any](ops ...Option) (SignalFlow[Message], error) 
```

### `OptionWithHost(v string)`

Sets the host of the RMQ Broker for Signal-Flow. This option is required.

### `OptionWithCodec(c Codec)`

The Signal-Flow library defaults to using the JSON Codec. However, you can utilize other Codecs defined in the [codec](https://github.com/harpy-wings/signal-flow/tree/main/codec) directory or define your own Codec. It just needs to implement the `Codec` interface.

```go
// Codec is an interface for encoding and decoding messages.
// By default, two codecs (json, binary) are available under the codec package.
type Codec interface {
  // Encode encodes the message to an array of bytes.
  Encode(m any) ([]byte, error)

  // Decode retrieves the message from an array of bytes.
  Decode(any, []byte) error

  // ContentType returns the content type of the codec.
  // Example: "application/json"
  ContentType() string
}
```

### `OptionWithName(v string)`

This function specifies a name for the consumer. By default, Signal-Flow picks up a random name starting with the `sf_` prefix. However, it's better for the developer to provide a proper name so it can be easily identified for logging or in the RMQ management dashboard.

### `OptionWithQueueName(v string)`

This option specifies a queue name for Signal-Flow. Once the queue name is specified, Signal-Flow initializes the consumer.

### `OptionWithExchangeName(v string)`

This option specifies the exchange name for Signal-Flow. Once the exchange name is specified, Signal-Flow initializes the producer.

### `OptionWithRoutingKey(v string)`

This option is used to specify the default routing key for the producer. If you set a routing key and the exchange name is empty, Signal-Flow initializes the producer for the default exchange and uses the routing key. You can pass the queue name here with the default routing key to emit a message to a specific queue.

### `OptionWithErrorHandler(fn func(error))`

Since some processes happen in parallel, errors may occur while processing messages. This function allows you to handle those errors for better error handling.

### `OptionQueueDeclaration(req QueueDeclarationRequest)`

By default, Signal-Flow does not create a queue if it does not exist when attempting to consume from it. If you want to declare a queue, you can use this option. Queue declaration occurs as soon as the connection is established and before the consumer initializes.

### `OptionExchangeDeclaration(req ExchangeDeclarationRequest)`

By default, Signal-Flow does not create an exchange if it does not exist when trying to emit a message to it. If you want to declare an exchange, you can use this option. Exchange declaration occurs as soon as the connection is established and before the producer initializes.

### `OptionBinding(req BindingRequest)`

You can bind an exchange to another exchange or queue using this option. It will be executed once the connection is established and before the consumer or producer initializes.
> Note: You need to specify the destination type of the binding, whether it's a queue or an exchange.

### `OptionWithLogger(v logrus.FieldLogger)`

This option allows you to set the logger for SignalFlow. SignalFlow will then utilize this logger for logging purposes, enabling you to specify your configuration for handling logs.

## Contributing

Contributions are welcome. Please read the [Contribution](https://github.com/harpy-wings/signal-flow/blob/main/README.md) document and raise an issue before starting work.

## Run the tests

Ensure that RMQP is available at `amqp://guest:guest@localhost:5672/` or your Docker daemon is running. Then run the tests using:

```shell
go test -v ./...
```

## Dependencies

- [github.com/sirupsen/logrus](https://github.com/sirupsen/logrus)
- [github.com/rabbitmq/amqp091-go](https://github.com/rabbitmq/amqp091-go)

### Standard Library Imports

- errors
- math
- math/rand
- strconv
- sync
- time

***

[![SonarCloud](https://sonarcloud.io/images/project_badges/sonarcloud-white.svg)](https://sonarcloud.io/summary/new_code?id=harpy-wings_signal-flow)
