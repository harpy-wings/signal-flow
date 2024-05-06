package main

import (
	"context"
	"log"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	signalflow "github.com/harpy-wings/signal-flow"
	"github.com/harpy-wings/signal-flow/codec"
)

func main() {
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()
	containerID := "e2030bd36b2b6fac250d0990b419a47bfe5f98d64fab68748cfec98b85004e70"
	err = apiClient.ContainerStart(context.Background(), containerID, container.StartOptions{})
	if err != nil {
		panic(err)
	}
	time.Sleep(8 * time.Second)

	type Telegraph struct {
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
	SF.ForeachN(func(m Telegraph) error {
		log.Println(m)
		return nil
	}, 1)
	// time.Sleep(3 * time.Second)
	err = SF.Emit(Telegraph{
		Text: "Hello Gopher 1!",
	})
	if err != nil {
		panic(err)
	}
	//expect to recive a Hello Gopher 1!

	containerTimeout := 0 // not wait
	err = apiClient.ContainerRestart(context.Background(), containerID, container.StopOptions{
		Timeout: &containerTimeout,
	})
	if err != nil {
		panic(err)
	}

	time.Sleep(20 * time.Second)
	log.Println("Sending Message 2")
	err = SF.Emit(Telegraph{
		Text: "Hello Gopher 2!",
	})
	if err != nil {
		panic(err)
	}
	// expect to receive the second message
	time.Sleep(60 * time.Second)
}
