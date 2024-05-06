package signalflow

import (
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

type signalFlow[Message any] struct {
	sync.Mutex
	logger logrus.FieldLogger
	codec  Codec

	amqConn *amqp.Connection

	RXC       <-chan amqp.Delivery
	amqRXChan *amqp.Channel
	rxFlow    struct {
		channelClosed chan *amqp.Error
	}

	TXC       chan amqp.Delivery
	amqTXChan *amqp.Channel
	txFlow    struct {
		channelClosed chan *amqp.Error
	}

	amqConnNotifyBlocked chan amqp.Blocking
	amqConnNotifyClose   chan *amqp.Error

	amqConsumeAttempts  int
	amqProducerAttempts int
	amqConnAttempts     int

	config struct {
		host                  string                 // RMQ host.
		name                  string                 // Name of the consumer. By default, a random number with "sg_" prefix.
		queueName             string                 // Queue name for the consumer to consume messages.
		exchangeName          string                 // Exchange name where the producer will push messages.
		routingKey            string                 // Routing key for the producer to emit messages into the exchange.
		exclusive             bool                   // Whether the consumer should be exclusive or not. Default is false.
		flowControlBufferSize int                    // Buffer size of channel for flow control purposes. Default is 1.
		amqAttemptsLimit      int                    // Attempts limit for retrying.
		priority              uint8                  // Default is 0; range is 0 to 9.
		mandatory             bool                   // Default is false.
		immediate             bool                   // Default is false.
		args                  map[string]interface{} // Default arguments passed to RMQ server in every request.
		errorHandler          func(error)            // Function called to handle asynchronous errors.

		onConnectionStabilized []func(channel *amqp.Channel) error
	}
}

var _ SignalFlow[interface{}] = &signalFlow[interface{}]{}

func New[Message any](ops ...Option) (SignalFlow[Message], error) {
	factoryS := new(signalFlow[any])
	var err error

	factoryS.setDefaults()
	for _, fn := range ops {
		err = fn(factoryS)
		if err != nil {
			return nil, err
		}
	}
	s := (*signalFlow[Message])(factoryS)
	err = s.init()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (f *signalFlow[Message]) setDefaults() {
	f.config.flowControlBufferSize = defaultFlowControlBufferSize
	f.config.name = signalFlowPrefix + strconv.Itoa(rand.Intn(math.MaxInt))
	f.config.amqAttemptsLimit = defaultAmqAttemptsLimit
	{
		//using logirus as default logger
		defaultLogger := logrus.StandardLogger()
		defaultLogger.SetLevel(logrus.DebugLevel)
		f.logger = defaultLogger
	}

	f.config.errorHandler = func(err error) {
		f.logger.Error(err)
	}
}

func (f *signalFlow[Message]) init() error {
	var err error
	err = f.connect()
	if err != nil {
		return err
	}

	if len(f.config.onConnectionStabilized) > 0 {
		// setup configuration channel,
		C, err := f.amqConn.Channel()
		if err != nil {
			return err
		}
		for _, fn := range f.config.onConnectionStabilized {
			err = fn(C)
			if err != nil {
				return err
			}
		}
		C.Close()
	}

	if f.config.queueName != "" {
		// activate the receiver
		err = f.consume()
		if err != nil {
			return err
		}
	}

	if f.config.exchangeName != "" {
		err = f.initProducer()
		if err != nil {
			return err
		}
	}

	return nil
}

// ForeachN is bounded parallelism pattern, it executes the `fn` function for each message in `n` goroutines. Acks the RMQ if and only if the `fn` returns nil.
func (f *signalFlow[Message]) ForeachN(fn func(Message) error, n int) error {
	// Preconditions
	if f.RXC == nil {
		return ErrNilChannel
	}

	// create the goroutines
	for i := 0; i < n; i++ {
		go func() {
			for {
				// once the channel is closed the for range will break the look, so we keep this loop with the hope of connection retries.
				for msg := range f.RXC {
					err := f.handleDeliveredMsg(fn, msg)
					if err != nil {
						f.config.errorHandler(err)
					}
				}
				time.Sleep(time.Second)
			}
		}()
	}

	return nil
}

// Emit is sending the message to all registered exchanges. if it fails to send the message, will return an error. Otherwise it will return nil.
func (f *signalFlow[Message]) Emit(msg Message) error {
	bs, err := f.codec.Encode(msg)
	if err != nil {
		return err
	}
	//todo add attempts pattern for Emit
	err = f.amqTXChan.Publish(
		f.config.exchangeName,
		f.config.routingKey,
		f.config.mandatory,
		f.config.immediate,
		amqp.Publishing{
			ContentType: f.codec.ContentType(),
			Body:        bs,
			Timestamp:   time.Now(),
			Priority:    f.config.priority,
			Headers:     f.config.args,
		},
	)
	if err != nil {
		return err
	}
	return nil
}

// handleDeliveredMsg : internal function that executes for each message received.
func (f *signalFlow[Message]) handleDeliveredMsg(fn func(Message) error, msg amqp.Delivery) error {
	// decode the message
	var Msg Message
	err := f.codec.Decode(&Msg, msg.Body)
	if err != nil {
		return err
	}

	// process the message
	err = fn(Msg)
	if err != nil {
		return err
	}

	// Send Ack
	err = msg.Ack(true)
	if err != nil {
		return err
	}
	return nil
}

// connect tires to connect the the best RMQ host.
// returns error if max attempts are reached.
func (f *signalFlow[Message]) connect() error {
	var err error

	// dial the Rmq Broker
	f.amqConn, err = amqp.Dial(f.config.host)
	if err != nil {
		time.Sleep(time.Duration(f.amqConnAttempts) * time.Second)
		f.amqConnAttempts++
		if f.amqConnAttempts > f.config.amqAttemptsLimit {
			return err
		}
		f.logger.Warnf("amqp connection error: %s; %d attempts to retrieving connection.", err.Error(), f.amqConnAttempts)
		return f.connect()
	}
	f.amqConnAttempts = 0

	f.Lock()
	defer f.Unlock()
	f.amqConnNotifyBlocked = f.amqConn.NotifyBlocked(make(chan amqp.Blocking, f.config.flowControlBufferSize))
	f.amqConnNotifyClose = f.amqConn.NotifyClose(make(chan *amqp.Error, f.config.flowControlBufferSize))

	go func() {
		select {
		case b := <-f.amqConnNotifyBlocked:
			f.connNotifyBlockedHandler(b)
		case e := <-f.amqConnNotifyClose:
			f.connNotifyCloseHandler(e)
		}
	}()
	f.logger.Debugf("SignalFlow connection established for %s.", f.config.name)
	return nil
}

// connNotifyBlockedHandler : handle the connection blocked notification. it logs the reason and tries to retrieve the connection.
func (f *signalFlow[Message]) connNotifyBlockedHandler(b amqp.Blocking) {
	f.logger.Errorf("RMQ broker blocked the connection with reason: %s", b.Reason)
	err := f.connect()
	if err != nil {
		f.config.errorHandler(err)
		f.logger.Fatal(err)
	}
}

// connNotifyCloseHandler : handle the connection closed notification. it logs the reason and tries to retrieve the connection.
func (f *signalFlow[Message]) connNotifyCloseHandler(e *amqp.Error) {
	f.logger.Warnf("notification received since connection closed: %s; retrieving the connection", e.Error())
	err := f.connect()
	if err != nil {
		f.config.errorHandler(err)
		f.logger.Fatal(err)
	}
}

// channelNotifyClosed handle the rmq channel closed notifications.
// it tries to retrieve the channel again by calling recoverFn. NOTE: no the connection, only the channel.
func (f *signalFlow[Message]) channelNotifyClosed(c chan *amqp.Error, recoverFn func() error) {
	e, ok := <-c
	if e == nil || !ok {
		// skip and close the thread
		return
	}
	f.logger.Warnf("notification received since channel closed: %s; retrieving the RX channel", e.Error())
	err := recoverFn()
	if err != nil {
		// unexpected case, maybe the RMQ broker is down for long time.
		f.logger.Error(err)
		f.config.errorHandler(err)
	}
}

// consume starts consuming from the queue
// if the connection is closed, it retires f.config.amqAttemptsLimits and then return the error.
func (f *signalFlow[Message]) consume() error {
	var err error
	if f.amqConn.IsClosed() {
		time.Sleep(time.Duration(f.amqConsumeAttempts) * time.Second)
		f.amqConsumeAttempts++
		if f.amqConsumeAttempts > f.config.amqAttemptsLimit {
			return ErrConnectionClosed
		}
		return f.consume()
	}
	f.Lock()
	defer f.Unlock()
	f.amqConsumeAttempts = 0

	f.amqRXChan, err = f.amqConn.Channel()
	if err != nil {
		return err
	}
	f.rxFlow.channelClosed = f.amqRXChan.NotifyClose(make(chan *amqp.Error, f.config.flowControlBufferSize))
	defer func() { go f.channelNotifyClosed(f.rxFlow.channelClosed, f.consume) }()
	f.RXC, err = f.amqRXChan.Consume(f.config.queueName, f.config.name, false, f.config.exclusive, false, false, f.config.args)
	if err != nil {
		return err
	}

	f.logger.Infof("%s consumer stabilized for queue %s.", f.config.name, f.config.queueName)
	return nil
}

func (f *signalFlow[Message]) initProducer() error {
	var err error
	if f.amqConn.IsClosed() {
		time.Sleep(time.Duration(f.amqProducerAttempts) * time.Second)
		f.amqProducerAttempts++
		if f.amqProducerAttempts > f.config.amqAttemptsLimit {
			return ErrConnectionClosed
		}
		return f.initProducer()
	}
	f.Lock()
	defer f.Unlock()
	f.amqProducerAttempts = 0

	f.amqTXChan, err = f.amqConn.Channel()
	if err != nil {
		return err
	}
	// handle channel notify close
	f.txFlow.channelClosed = f.amqTXChan.NotifyClose(make(chan *amqp.Error, f.config.flowControlBufferSize))
	defer func() { go f.channelNotifyClosed(f.txFlow.channelClosed, f.initProducer) }()

	// f.amqTXChan.NotifyReturn()

	f.logger.Infof("%s producer stabilized for exchange %s.", f.config.name, f.config.exchangeName)
	return nil
}
