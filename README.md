# signal-flow
[![Go Report Card](https://goreportcard.com/badge/github.com/harpy-wings/signal-flow)](https://goreportcard.com/report/github.com/harpy-wings/signal-flow)
[![SonarCloud](https://sonarcloud.io/images/project_badges/sonarcloud-white.svg)](https://sonarcloud.io/summary/new_code?id=harpy-wings_signal-flow)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=harpy-wings_signal-flow&metric=coverage)](https://sonarcloud.io/summary/new_code?id=harpy-wings_signal-flow)
[![codecov](https://codecov.io/gh/harpy-wings/signal-flow/branch/main/graph/badge.svg?token=CSO8PJZ0NU)](https://codecov.io/gh/harpy-wings/signal-flow)
[![Reliability Rating](https://sonarcloud.io/api/project_badges/measure?project=harpy-wings_signal-flow&metric=reliability_rating)](https://sonarcloud.io/summary/new_code?id=harpy-wings_signal-flow)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=harpy-wings_signal-flow&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=harpy-wings_signal-flow)

SignalFlow is an library for handling AMQP operations, including retrying and reconnecting strategies. It facilitates message emission and retrieval from AMQP brokers,incorporating flow control signals. It is designed generically to support various data formats.

```txt
                Queue
                ↧ 1:n (Foreach)
+―――――――――――――――――――+
| SignalFlow        | 
| Foreach           |
| Emit              | ↦ Exchange
+―――――――――――――――――――+ 1:n (Emit)


```

```go
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
```
