# signal-flow

SignalFlow is a package for event driven development in golang using Message broker ( RMQ ).

```txt
                Queue
                ↧ 1:n (ForeachN)
+―――――――――――――――――――+
| SignalFlow        | 
| ForeachN          |
| Emit              | ↦ Exchange
+―――――――――――――――――――+ 1:n (Emit)


```

```go
interface SignalFlow {
    // ForeachN is bounded parallelism pattern, it executes the `fn` function for each message in `n` goroutines. Acks the RMQ if and only if the `fn` returns nil.
    ForeachN(fn func(Message) error,n int)

    // Emit is sending the message to all registered exchanges. if it fails to send the message, will return an error. Otherwise it will return nil.
    Emit(Message) error
}
```