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
	SF.ForeachN(func(m Message) error {
		log.Println(m)
		return nil
	}, 1)
	// time.Sleep(3 * time.Second)
	err = SF.Emit(Message{
		Command: 1,
		Value:   21, // 21 (10101)2 should be in the binary format
	})
	if err != nil {
		panic(err)
	}

	time.Sleep(20 * time.Second)

}
