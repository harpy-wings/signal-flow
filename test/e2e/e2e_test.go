package e2e_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	signalflow "github.com/harpy-wings/signal-flow"
	"github.com/harpy-wings/signal-flow/codec"
	"github.com/stretchr/testify/require"
)

const (
	NumberOfTestCases = 1000
)

func TestStory1(t *testing.T) {
	// set OS environment variables
	err := os.Setenv("DOCKER_API_VERSION", "1.44")
	require.NoError(t, err)

	// setup the Docker client
	docker, err := client.NewClientWithOpts(client.FromEnv)
	require.NoError(t, err)
	defer docker.Close()
	var containerID string
	containerID = "e2030bd36b2b6fac250d0990b419a47bfe5f98d64fab68748cfec98b85004e70"
	if containerID == "" {
		// setup the RMQ container

		containerResp, err := docker.ContainerCreate(context.Background(), &container.Config{
			Image: "rabbitmq:3.13-management",
			ExposedPorts: nat.PortSet{
				"15672": {},
				"5672":  {},
			},
		}, &container.HostConfig{
			AutoRemove: true,
			PortBindings: nat.PortMap{
				"15672": []nat.PortBinding{{"0.0.0.0", "15672"}},
				"5672":  []nat.PortBinding{{"0.0.0.0", "5672"}},
			},
		}, nil, nil, "signal-flow_RMQ_e2e_test")
		require.NoError(t, err)
		containerID = containerResp.ID
	}

	// start the rmq server
	err = docker.ContainerStart(context.Background(), containerID, container.StartOptions{})
	require.NoError(t, err)

	defer func() {
		// docker.ContainerStop(context.Background(), containerID, container.StopOptions{})
	}()
	// waiting for the container to be started, it takes about 7 seconds.
	time.Sleep(10 * time.Second)

	// defining type of message
	type Message struct {
		UID string `json:"uid"`
		Age int    `json:"age"`
	}
	var (
		QueueName    = "test-queue-1"
		ExchangeName = "exchange-1"
	)

	// Creating the SignalFlow client.
	SF, err := signalflow.New[Message](
		signalflow.OptionWithHost("amqp://guest:guest@localhost:5672/"),
		signalflow.OptionWithQueueName(QueueName),        // enable the consumer
		signalflow.OptionWithExchangeName(ExchangeName),  // enable the producer.
		signalflow.OptionWithCodec(codec.NewJsonCodec()), // use the JSON codec
		signalflow.OptionQueueDeclaration(signalflow.QueueDeclarationRequest{
			Name:       QueueName,
			Durable:    true,
			AutoDelete: false,
			Elusive:    false,
			NoWait:     false,
		}),
		signalflow.OptionQueueDeclaration(signalflow.QueueDeclarationRequest{
			Name:       "debug-queue",
			Durable:    true,
			AutoDelete: false,
			Elusive:    false,
			NoWait:     false,
		}),
		signalflow.OptionExchangeDeclaration(signalflow.ExchangeDeclarationRequest{
			Name:    ExchangeName,
			Kind:    "fanout",
			Durable: true,
		}),
		signalflow.OptionBinding(signalflow.BindingRequest{
			Source:      ExchangeName,
			Destination: QueueName,
			// DestinationType: signalflow.BindingDestinationTypeQueue, as default
		}),
		signalflow.OptionBinding(signalflow.BindingRequest{
			Source:      ExchangeName,
			Destination: "debug-queue",
			// DestinationType: signalflow.BindingDestinationTypeQueue, as default
		}),
		// signalflow.OptionWithGlobalQoS(10, 10),
		signalflow.OptionWithErrorHandler(func(err error) {
			require.NoError(t, err)
		}),
	) // New
	require.NoError(t, err)

	testCases := make(map[string]bool)
	testCaseLock := sync.Mutex{}

	// emit all test cases
	for i := 0; i < NumberOfTestCases; i++ {
		uid := uuid.NewString()
		err = SF.Emit(Message{
			UID: uid,
			Age: i,
		})
		require.NoError(t, err)
		testCaseLock.Lock()
		testCases[uid] = false
		testCaseLock.Unlock()
	}
	t.Log("test cases emitted")

	err = SF.Foreach(func(m Message) error {
		t.Logf("message received: %+v\n", m)
		testCaseLock.Lock()
		testCases[m.UID] = true // Message received
		testCaseLock.Unlock()
		time.Sleep(10 * time.Millisecond) // lets make it slow, so consuming all the messages will take NumberOfTestCases/2*100 Seconds(25 Seconds.)
		return nil
	})
	require.NoError(t, err)

	time.Sleep(5 * time.Second)
	// lets make the RMQ server unexpected crash but restored successfully.
	t.Log("simulating the network failure")
	{
		err := docker.ContainerPause(context.Background(), containerID)
		require.NoError(t, err)
		time.Sleep(5 * time.Second)
		err = docker.ContainerUnpause(context.Background(), containerID)
		require.NoError(t, err)
		// Restarting the container will cause to unread messages be deleted
		// err = docker.ContainerRestart(context.Background(), containerID, container.StopOptions{})
		// resp, err := docker.ContainerExecCreate(context.Background(), containerID, types.ExecConfig{
		// 	Cmd: []string{"rabbitmqctl", "close_all_connections"},
		// })
		// require.NoError(t, err)
		// err = docker.ContainerExecStart(context.Background(), resp.ID, types.ExecStartCheck{})
		// require.NoError(t, err)
	}

	// it will take about 20 seconds for SignalFlow to reestablished every thing.
	time.Sleep((15) * time.Second)

	// let's see if emitter is still working.
	t.Logf("Sending few more messages\n")
	for i := 0; i < NumberOfTestCases/100; i++ {
		uid := uuid.NewString()
		err = SF.Emit(Message{
			UID: uid,
			Age: i,
		})
		require.NoError(t, err)
		testCaseLock.Lock()
		testCases[uid] = false
		testCaseLock.Unlock()
	}
	// let's wait for all of the messages consume.
	time.Sleep(20 * time.Second)

	// let's verify the test cases
	unexpectedCase := 0
	unexpectedUID := ""
	for k, v := range testCases {
		if !v {
			unexpectedUID = k
			unexpectedCase++
		}
	}
	require.Equal(t, 0, unexpectedCase, "expected all messages consume but some item failed; ex:", unexpectedUID)

}