package signalflow

import (
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/harpy-wings/signal-flow/test/mocks"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/require"
)

//generate dependencies
//go:generate mockgen -destination=./test/mocks/acknowledger.go -package=mocks github.com/streadway/amqp Acknowledger

func TestNew(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		SF, err := New[interface{}](OptionWithHost(localAMQPHost), OptionWithQueueName("q_test"), OptionWithExchangeName("ex_test"), func(sf *signalFlow[any]) error {
			sf.config.onConnectionStabilized = append(sf.config.onConnectionStabilized, func(channel *amqp.Channel) error {
				_, err := channel.QueueDeclare("q_test", false, false, false, false, nil)
				if err != nil {
					return err
				}
				err = channel.ExchangeDeclare("ex_test", "fanout", false, false, false, false, nil)
				if err != nil {
					return err
				}
				return nil
			})
			return nil
		})
		require.NoError(t, err)
		require.NotNil(t, SF)
		t.Run("flow control", func(t *testing.T) {
			sf := SF.(*signalFlow[interface{}])
			err := sf.amqConn.Close()
			// expect to retrieve the connection.
			require.NoError(t, err)
			time.Sleep(500 * time.Millisecond)

			sf.amqConnNotifyBlocked <- amqp.Blocking{Active: false, Reason: "test flow control"} // expect to retrieve the connection
			time.Sleep(500 * time.Millisecond)
		})
		t.Run("consumer", func(t *testing.T) {
			sf := SF.(*signalFlow[interface{}])
			c, err := sf.amqConn.Channel()
			require.NoError(t, err)
			_ = c

		})

	})

	t.Run("failure", func(t *testing.T) {
		t.Run("connect", func(t *testing.T) {
			SF, err := New[interface{}](OptionWithHost("amqp://guest:guest@localhost:2765/"), func(sf *signalFlow[any]) error { sf.config.amqAttemptsLimit = 1; return nil })
			require.Error(t, err)
			require.Nil(t, SF)
		})
		t.Run("options", func(t *testing.T) {
			SF, err := New[interface{}](func(sf *signalFlow[any]) error { return errors.New("any") })
			require.Error(t, err)
			require.Nil(t, SF)
		})

		t.Run("init", func(t *testing.T) {
			SF, err := New[interface{}](OptionWithHost(localAMQPHost), func(sf *signalFlow[any]) error {
				sf.config.onConnectionStabilized = append(sf.config.onConnectionStabilized, func(channel *amqp.Channel) error {
					return errors.New("any")
				})
				return nil
			})
			require.Error(t, err)
			require.Nil(t, SF)
		})

	})
}

func TestEmit(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		SF, err := New[interface{}](OptionWithHost(localAMQPHost), OptionWithExchangeName("ex_test"), func(sf *signalFlow[any]) error {
			sf.config.onConnectionStabilized = append(sf.config.onConnectionStabilized, func(channel *amqp.Channel) error {
				err := channel.ExchangeDeclare("ex_test", "fanout", false, false, false, false, nil)
				if err != nil {
					return err
				}
				return nil
			})
			return nil
		})
		require.NoError(t, err)
		require.NotNil(t, SF)

		codec := mocks.NewMockCodec(gomock.NewController(t))
		sf := SF.(*signalFlow[interface{}])
		sf.codec = codec
		codec.EXPECT().Encode(gomock.Any()).Return([]byte("Hello, world!"), nil).AnyTimes()
		codec.EXPECT().ContentType().Return("application/text").AnyTimes()

		err = SF.Emit("Hello World!")
		require.NoError(t, err)

		t.Run("test Flow Lock", func(t *testing.T) {
			t.Run("pause and resume flow", func(t *testing.T) {
				t.Parallel()
				sf.txFlow.notifyFlow <- true // pause
				time.Sleep(time.Second)
				sf.txFlow.notifyFlow <- false // resume
			})
			t.Run("emit", func(t *testing.T) {
				t.Parallel()
				time.Sleep(100 * time.Millisecond)
				n := time.Now()
				err := SF.Emit("Hello World with delay!")
				require.NoError(t, err)
				require.Greater(t, time.Since(n).Milliseconds(), int64(300)) // so the Emit function is locked until resume signal received
			})
		})
	})

	t.Run("failure", func(t *testing.T) {
		t.Run("codec failure", func(t *testing.T) {
			SF, err := New[interface{}](OptionWithHost(localAMQPHost), OptionWithExchangeName("ex_test"), func(sf *signalFlow[any]) error {
				sf.config.onConnectionStabilized = append(sf.config.onConnectionStabilized, func(channel *amqp.Channel) error {
					err := channel.ExchangeDeclare("ex_test", "fanout", false, false, false, false, nil)
					if err != nil {
						return err
					}
					return nil
				})
				return nil
			})
			require.NoError(t, err)
			require.NotNil(t, SF)

			codec := mocks.NewMockCodec(gomock.NewController(t))
			sf := SF.(*signalFlow[interface{}])
			sf.codec = codec
			codec.EXPECT().Encode(gomock.Any()).Return([]byte{}, errors.New("any")).AnyTimes()
			codec.EXPECT().ContentType().Return("application/text").AnyTimes()

			err = SF.Emit("Hello World!")
			require.Error(t, err)
		})

		t.Run("intiProducer", func(t *testing.T) {
			sf := &signalFlow[interface{}]{}
			conn, err := amqp.Dial(localAMQPHost)
			require.NoError(t, err)
			conn.Close()

			sf.config.amqAttemptsLimit = 0
			sf.amqProducerAttempts = 1
			sf.amqConn = conn
			err = sf.initProducer()
			require.Error(t, err)
		})
	})
}

func TestForeach(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		SF, err := New[interface{}](OptionWithHost(localAMQPHost), OptionWithQueueName("q_test"), func(sf *signalFlow[any]) error {
			sf.config.onConnectionStabilized = append(sf.config.onConnectionStabilized, func(channel *amqp.Channel) error {
				_, err := channel.QueueDeclare("q_test", false, false, false, false, nil)
				if err != nil {
					return err
				}
				return nil
			})
			return nil
		})
		require.NoError(t, err)
		require.NotNil(t, SF)

		codec := mocks.NewMockCodec(gomock.NewController(t))
		sf := SF.(*signalFlow[interface{}])
		sf.codec = codec
		codec.EXPECT().Decode(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		codec.EXPECT().ContentType().Return("application/text").AnyTimes()
		acknowledger := mocks.NewMockAcknowledger(gomock.NewController(t))
		acknowledger.EXPECT().Ack(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		signal := make(chan struct{})
		err = sf.Foreach(func(a any) error {
			signal <- struct{}{}
			return nil
		})
		require.NoError(t, err)
		rxc := make(chan amqp.Delivery)
		go func() {
			sf.rxcProxy(rxc)
		}()
		rxc <- amqp.Delivery{
			Acknowledger: acknowledger,
			DeliveryTag:  0,
			Body:         []byte("Hello World!"),
		}
		close(rxc)
		<-signal // wait until message precess

	})

	t.Run("failure", func(t *testing.T) {
		t.Run("consume", func(t *testing.T) {

			sf := &signalFlow[interface{}]{}
			conn, err := amqp.Dial(localAMQPHost)
			require.NoError(t, err)
			conn.Close()
			sf.config.amqAttemptsLimit = 0
			sf.amqConsumeAttempts = 1
			sf.amqConn = conn
			err = sf.consume()
			require.Error(t, err)
		})
	})

}

func TestChannelNotifyClose(t *testing.T) {
	sf := &signalFlow[interface{}]{}
	sf.config.errorHandler = func(err error) {}
	sf.logger = testLogger
	c := make(chan *amqp.Error)
	t.Run("notify signal", func(t *testing.T) {
		t.Parallel()
		c <- &amqp.Error{Reason: "test"}
	})
	t.Run("flow", func(t *testing.T) {
		t.Parallel()
		sf.channelNotifyClosed(c, func() error { return errors.New("not recoverable") })
	})
}
