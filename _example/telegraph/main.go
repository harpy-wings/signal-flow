package main

import (
	"log"
	"time"

	signalflow "github.com/harpy-wings/signal-flow"
	"github.com/harpy-wings/signal-flow/codec"
)

func main() {
	type Telegraph struct {
		Age  int
		Text string `json:"text"`
	}

	SF, err := signalflow.New[Telegraph](
		signalflow.OptionWithHost("amqp://guest:guest@localhost:5672/"),
		signalflow.OptionWithQueueName("qt1"),
		signalflow.OptionWithExchangeName("ext1"),
		signalflow.OptionWithCodec(codec.NewJsonCodec()),
		// signalflow.OptionWithCodec(codec.NewABICodec()),
	)
	if err != nil {
		panic(err)
	}

	SF.Foreach(func(m Telegraph) error {
		log.Println(m)
		m.Text
		return nil
	})

	err = SF.Emit(Telegraph{
		Text: "Hello Gopher 1!",
	})
	if err != nil {
		panic(err)
	}

	//expect to receive a Hello Gopher 1!
	err = SF.Emit(Telegraph{
		Text: "Hello Gopher 2!",
	})
	if err != nil {
		panic(err)
	}
	// expect to receive the second message
	time.Sleep(1 * time.Second)
}
