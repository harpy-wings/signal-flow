package signalflow

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/harpy-wings/signal-flow/test/mocks"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const (
	localAMQPHost = "amqp://guest:guest@localhost:5672/"
)

var (
	testLogger = logrus.StandardLogger()
)

func TestMain(m *testing.M) {

	if conn, err := amqp.Dial(localAMQPHost); err != nil {
		// local amqp is not available for testing.
		// using docker to create it.
		initDockerContainer()
	} else {
		conn.Close() // ready to go
	}
	os.Exit(m.Run())
}
func TestOptionQueueDeclaration(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sf := &signalFlow[interface{}]{
			logger: testLogger,
		}
		conn, err := amqp.Dial(localAMQPHost)
		require.NoError(t, err)
		defer conn.Close()

		amqChan, err := conn.Channel()
		require.NoError(t, err)

		err = OptionQueueDeclaration(QueueDeclarationRequest{Name: uuid.NewString()})(sf)
		require.NoError(t, err)
		for _, fn := range sf.config.onConnectionStabilized {
			err = fn(amqChan)
			require.NoError(t, err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		sf := &signalFlow[interface{}]{
			logger: testLogger,
		}
		conn, err := amqp.Dial(localAMQPHost)
		require.NoError(t, err)
		defer conn.Close()

		amqChan, err := conn.Channel()
		require.NoError(t, err)
		amqChan.Close() // channel closed so it should return an error
		err = OptionQueueDeclaration(QueueDeclarationRequest{Name: uuid.NewString()})(sf)
		require.NoError(t, err)
		for _, fn := range sf.config.onConnectionStabilized {
			err = fn(amqChan)
			require.Error(t, err)
		}
	})

}

func TestOptionExchangeDeclaration(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sf := &signalFlow[interface{}]{
			logger: testLogger,
		}
		conn, err := amqp.Dial(localAMQPHost)
		require.NoError(t, err)
		defer conn.Close()
		amqChan, err := conn.Channel()
		require.NoError(t, err)

		err = OptionExchangeDeclaration(ExchangeDeclarationRequest{Name: uuid.NewString(), Kind: "direct"})(sf)
		require.NoError(t, err)
		for _, fn := range sf.config.onConnectionStabilized {
			err = fn(amqChan)
			require.NoError(t, err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		sf := &signalFlow[interface{}]{
			logger: testLogger,
		}
		conn, err := amqp.Dial(localAMQPHost)
		require.NoError(t, err)
		defer conn.Close()

		amqChan, err := conn.Channel()
		require.NoError(t, err)
		amqChan.Close() // channel closed so it should return an error
		err = OptionExchangeDeclaration(ExchangeDeclarationRequest{Name: uuid.NewString(), Kind: "direct"})(sf)
		require.NoError(t, err)
		for _, fn := range sf.config.onConnectionStabilized {
			err = fn(amqChan)
			require.Error(t, err)
		}
	})

}

func TestOptionBinding(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sf := &signalFlow[interface{}]{
			logger: testLogger,
		}
		conn, err := amqp.Dial(localAMQPHost)
		require.NoError(t, err)
		defer conn.Close()

		amqChan, err := conn.Channel()
		require.NoError(t, err)
		testKey := uuid.NewString()

		err = amqChan.ExchangeDeclare("ex_"+testKey, "direct", false, false, false, false, nil)
		require.NoError(t, err)

		err = amqChan.ExchangeDeclare("ex_"+testKey+"_d", "direct", false, false, false, false, nil)
		require.NoError(t, err)

		_, err = amqChan.QueueDeclare("qu_"+testKey, false, false, false, false, nil)
		require.NoError(t, err)

		err = OptionBinding(BindingRequest{Source: "ex_" + testKey, Destination: "qu_" + testKey})(sf)
		require.NoError(t, err)

		err = OptionBinding(BindingRequest{Source: "ex_" + testKey, Destination: "ex_" + testKey + "_d", DestinationType: BindingDestinationTypeExchange})(sf)
		require.NoError(t, err)

		// unsupported binding type
		err = OptionBinding(BindingRequest{Source: "ex_" + testKey, Destination: "ex_" + testKey + "_d", DestinationType: 3})(sf)
		require.Error(t, err)
		for _, fn := range sf.config.onConnectionStabilized {
			err = fn(amqChan)
			require.NoError(t, err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		sf := &signalFlow[interface{}]{
			logger: testLogger,
		}
		conn, err := amqp.Dial(localAMQPHost)
		require.NoError(t, err)
		defer conn.Close()

		amqChan, err := conn.Channel()
		require.NoError(t, err)
		testKey := uuid.NewString()
		err = amqChan.Close()
		require.NoError(t, err)
		err = OptionBinding(BindingRequest{Source: "ex_" + testKey, Destination: "qu_" + testKey})(sf)
		require.NoError(t, err)

		err = OptionBinding(BindingRequest{Source: "ex_" + testKey, Destination: "ex_" + testKey + "_d", DestinationType: BindingDestinationTypeExchange})(sf)
		require.NoError(t, err)

		for _, fn := range sf.config.onConnectionStabilized {
			err = fn(amqChan)
			require.Error(t, err)
		}
	})

}

func TestOptionWithGlobalQoS(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sf := &signalFlow[interface{}]{
			logger: testLogger,
		}
		conn, err := amqp.Dial(localAMQPHost)
		require.NoError(t, err)
		defer conn.Close()

		amqChan, err := conn.Channel()
		require.NoError(t, err)

		err = OptionWithGlobalQoS(0, 0)(sf)
		require.NoError(t, err)
		for _, fn := range sf.config.onConnectionStabilized {
			err = fn(amqChan)
			require.NoError(t, err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		sf := &signalFlow[interface{}]{
			logger: testLogger,
		}
		conn, err := amqp.Dial(localAMQPHost)
		require.NoError(t, err)
		defer conn.Close()

		amqChan, err := conn.Channel()
		require.NoError(t, err)
		amqChan.Close() // channel closed so it should return an error
		err = OptionWithGlobalQoS(0, 0)(sf)
		require.NoError(t, err)
		for _, fn := range sf.config.onConnectionStabilized {
			err = fn(amqChan)
			require.Error(t, err)
		}
	})

}

func TestOptionWithQueueName(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sf := &signalFlow[interface{}]{}
		err := OptionWithQueueName("TestOptionWithQueueName")(sf)
		require.NoError(t, err)
		require.Equal(t, "TestOptionWithQueueName", sf.config.queueName)
	})
}

func TestOptionWithExchangeName(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sf := &signalFlow[interface{}]{}
		err := OptionWithExchangeName("TestOptionWithExchangeName")(sf)
		require.NoError(t, err)
		require.Equal(t, "TestOptionWithExchangeName", sf.config.exchangeName)
	})
}

func TestOptionWithHost(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sf := &signalFlow[interface{}]{}
		err := OptionWithHost("TestOptionWithHost")(sf)
		require.NoError(t, err)
		require.Equal(t, "TestOptionWithHost", sf.config.host)
	})
}

func TestOptionWithName(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sf := &signalFlow[interface{}]{}
		err := OptionWithName("TestOptionWithName")(sf)
		require.NoError(t, err)
		require.Equal(t, "TestOptionWithName", sf.config.name)
	})
}

func TestOptionWithCodec(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sf := &signalFlow[interface{}]{}
		cont := gomock.NewController(t)
		testCodec := mocks.NewMockCodec(cont)
		err := OptionWithCodec(testCodec)(sf)
		require.NoError(t, err)
		require.Equal(t, testCodec, sf.codec)
	})
}

func TestOptionWithRoutingKey(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sf := &signalFlow[interface{}]{}
		err := OptionWithRoutingKey("TestOptionWithRoutingKey")(sf)
		require.NoError(t, err)
		require.Equal(t, "TestOptionWithRoutingKey", sf.config.routingKey)
	})
}

func TestOptionWithErrorHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sf := &signalFlow[interface{}]{}
		err := OptionWithErrorHandler(func(err error) {})(sf)
		require.NoError(t, err)
	})

}

func initDockerContainer() {
	// set OS environment variables
	err := os.Setenv("DOCKER_API_VERSION", "1.43")
	if err != nil {
		panic(err)
	}

	// setup the Docker client
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}
	defer docker.Close()
	var containerID string
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
		if err != nil {
			panic(err)
		}
		containerID = containerResp.ID
	}

	// start the rmq server
	err = docker.ContainerStart(context.Background(), containerID, container.StartOptions{})
	if err != nil {
		panic(err)
	}
	time.Sleep(7 * time.Second)
}

func TestOptionWithLogger(t *testing.T) {
	l := logrus.New()
	sf := &signalFlow[interface{}]{}
	err := OptionWithLogger(l)(sf)
	require.NoError(t, err)
	require.NotNil(t, sf.logger)
}
