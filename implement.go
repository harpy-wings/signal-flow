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
	logger logrus.FieldLogger
	codec  Codec

	amqConn *amqp.Connection

	// rxc is the internal queue for handling the messages, created since RMQ RXC will lock the goroutines if the connection close unexpected.
	rxc chan amqp.Delivery
	// RXC       <-chan amqp.Delivery

	amqRXChan *amqp.Channel
	rxFlow    struct {
		lock          sync.Mutex
		channelClosed chan *amqp.Error
	}

	TXC       chan amqp.Delivery
	amqTXChan *amqp.Channel
	txFlow    struct {
		lock          sync.Mutex
		channelClosed chan *amqp.Error
		returnError   chan *amqp.Error

		notifyFlow chan bool
	}

	amqConnNotifyBlocked chan amqp.Blocking
	amqConnNotifyClose   chan *amqp.Error

	amqConsumeAttempts  int
	amqProducerAttempts int
	amqConnAttempts     int

	config struct {
		host                      string                 // RMQ host.
		name                      string                 // Name of the consumer. By default, a random number with "sg_" prefix.
		queueName                 string                 // Queue name for the consumer to consume messages.
		exchangeName              string                 // Exchange name where the producer will push messages.
		routingKey                string                 // Routing key for the producer to emit messages into the exchange.
		exclusive                 bool                   // Whether the consumer should be exclusive or not. Default is false.
		flowControlBufferSize     int                    // Buffer size of channel for flow control purposes. Default is 1.
		amqAttemptsLimit          int                    // Attempts limit for retrying.
		priority                  uint8                  // Default is 0; range is 0 to 9.
		mandatory                 bool                   // Default is false.
		immediate                 bool                   // Default is false.
		args                      map[string]interface{} // Default arguments passed to RMQ server in every request.
		errorHandler              func(error)            // Function called to handle asynchronous errors.
		numberOfGoRoutinesForeach int
		onConnectionStabilized    []func(channel *amqp.Channel) error
		ackTimeout                time.Duration
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
		// err = C.Close()
		if err != nil {
			return err
		}
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
func (f *signalFlow[Message]) Foreach(fn func(Message) error) error {
	// Preconditions
	if f.rxc == nil {
		return ErrNilChannel
	}

	// create the goroutines
	go func() {
		for msg := range f.rxc {
			err := f.handleDeliveredMsg(fn, msg)
			if err != nil {
				f.config.errorHandler(err)
			}
		}
	}()

	return nil
}

// Emit is sending the message to all registered exchanges. if it fails to send the message, will return an error. Otherwise it will return nil.
func (f *signalFlow[Message]) Emit(msg Message) error {
	return f.EmitR(msg, f.config.routingKey)
}

// Emit is sending the message to all registered exchanges. if it fails to send the message, will return an error. Otherwise it will return nil.
func (f *signalFlow[Message]) EmitR(msg Message, routingKey string) error {
	bs, err := f.codec.Encode(msg)
	if err != nil {
		return err
	}
	//todo add attempts pattern for Emit
	f.txFlow.lock.Lock()
	err = f.amqTXChan.Publish(
		f.config.exchangeName,
		routingKey,
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
	f.txFlow.lock.Unlock()
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
	go func() {
		err := msg.Ack(true)
		if err != nil {
			f.config.errorHandler(err)
		}
	}()

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
	f.logger.Warnf("notification received since channel closed: %s; retrieving the channel", e.Error())
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
	f.rxFlow.lock.TryLock() // avoid message acknowledgement process.
	var err error

	if f.rxc == nil {
		f.rxc = make(chan amqp.Delivery)
	}

	if f.amqConn.IsClosed() {
		time.Sleep(time.Duration(f.amqConsumeAttempts) * time.Second)
		f.amqConsumeAttempts++
		if f.amqConsumeAttempts > f.config.amqAttemptsLimit {
			return ErrConnectionClosed
		}
		return f.consume()
	}

	f.amqConsumeAttempts = 0

	f.amqRXChan, err = f.amqConn.Channel()
	if err != nil {
		return err
	}
	f.rxFlow.lock.Unlock()

	RXC, err := f.amqRXChan.Consume(f.config.queueName, f.config.name, false, f.config.exclusive, false, false, f.config.args)
	if err != nil {
		return err
	}
	f.rxFlow.channelClosed = f.amqRXChan.NotifyClose(make(chan *amqp.Error, f.config.flowControlBufferSize))

	go f.channelNotifyClosed(f.rxFlow.channelClosed, f.consume)
	// f.rxFlow.chanForeachNRecover <- struct{}{} //
	go f.rxcProxy(RXC)

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

	f.amqProducerAttempts = 0

	f.amqTXChan, err = f.amqConn.Channel()
	if err != nil {
		return err
	}
	// f.amqTXChan.NotifyReturn()

	// handle channel notify close
	f.txFlow.channelClosed = f.amqTXChan.NotifyClose(make(chan *amqp.Error, f.config.flowControlBufferSize))
	go f.channelNotifyClosed(f.txFlow.channelClosed, f.initProducer)

	f.txFlow.notifyFlow = f.amqTXChan.NotifyFlow(make(chan bool, f.config.flowControlBufferSize))
	go f.channelTXNotifyFlowHandler()
	f.logger.Infof("%s producer stabilized for exchange %s.", f.config.name, f.config.exchangeName)
	return nil
}

func (f *signalFlow[Message]) channelTXNotifyFlowHandler() {
	for flowSignal := range f.txFlow.notifyFlow {
		if flowSignal {
			f.logger.Infof("signal flow producer got pause command from RMQ server")
			f.txFlow.lock.Lock()
		} else {
			f.logger.Infof("signal flow producer got continue command from RMQ server")
			f.txFlow.lock.Unlock()
		}
	}
}

func (f *signalFlow[Message]) rxcProxy(RXC <-chan amqp.Delivery) {
	// start proxy consumer
	f.logger.Debugf("Proxy consumer started")
	for msg := range RXC {
		f.rxc <- msg
	}
	f.logger.Debugf("Proxy consumer ended")
}
