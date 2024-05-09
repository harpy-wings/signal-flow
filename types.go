package signalflow

type BindingDestinationType uint8

const (
	BindingDestinationTypeQueue    BindingDestinationType = iota // By default, we expect to bind an exchange to a queue.
	BindingDestinationTypeExchange                               // Binding an exchange to another exchange.
)

// QueueDeclarationRequest contains the configuration for a queue declaration request.
type QueueDeclarationRequest struct {
	Name       string                 `json:"name"`        // Name of the queue.
	Durable    bool                   `json:"durable"`     // Whether the queue is durable.
	AutoDelete bool                   `json:"auto_delete"` // Whether the queue should be auto-deleted.
	Elusive    bool                   `json:"elusive"`     // Whether the queue is exclusive.
	NoWait     bool                   `json:"no_wait"`     // Whether to wait for the declaration to complete.
	Args       map[string]interface{} `json:"args"`        // Additional arguments for queue declaration.
}

// ExchangeDeclarationRequest contains the configuration for an exchange declaration request.
type ExchangeDeclarationRequest struct {
	Name       string                 `json:"name"`        // Name of the exchange.
	Kind       string                 `json:"kind"`        // Type of the exchange (direct, topic, headers, fanout).
	Durable    bool                   `json:"durable"`     // Whether the exchange is durable.
	AutoDelete bool                   `json:"auto_delete"` // Whether the exchange should be auto-deleted.
	Internal   bool                   `json:"internal"`    // Whether the exchange is internal.
	NoWait     bool                   `json:"no_wait"`     // Whether to wait for the declaration to complete.
	Args       map[string]interface{} `json:"args"`        // Additional arguments for exchange declaration.
}

// BindingRequest contains the configuration for binding an exchange to another queue or exchange.
type BindingRequest struct {
	Source          string                 `json:"source"`           // Source exchange or queue.
	Destination     string                 `json:"destination"`      // Destination exchange or queue.
	DestinationType BindingDestinationType `json:"destination_type"` // Type of the destination (queue or exchange).
	RoutingKey      string                 `json:"routing_key"`      // Routing key for the binding.
	NoWait          bool                   `json:"no_wait"`          // Whether to wait for the binding to complete.
	Args            map[string]interface{} `json:"args"`             // Additional arguments for binding.
}
