package main

import (
	"log"
	"time"

	signalflow "github.com/harpy-wings/signal-flow"
	"github.com/harpy-wings/signal-flow/codec"
)

func main() {

	type Message struct {
		Command uint8
		Value   uint64
	}
	SF, err := signalflow.New[Message](
		signalflow.OptionWithHost("amqp://guest:guest@localhost:5672/"),
		signalflow.OptionWithQueueName("qt1"),
		signalflow.OptionWithExchangeName("ext1"),
		signalflow.OptionWithCodec(codec.NewABICodec()),
	)
	if err != nil {
		panic(err)
	}
	SF.Foreach(func(m Message) error {
		log.Printf("%+v", m)
		return nil
	})

	// time.Sleep(3 * time.Second)
	err = SF.Emit(Message{
		Command: 1,
		Value:   21, // 21 (10101)2 should be in the binary format
	})
	if err != nil {
		panic(err)
	}

	time.Sleep(1 * time.Second)
}
