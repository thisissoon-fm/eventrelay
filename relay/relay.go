package relay

import (
	"encoding/json"
	"errors"
	"io"
	"sync"

	"eventrelay/logger"
	"eventrelay/pubsub"
)

// Error vars
var (
	ErrTopicNotSupported  = errors.New("topic not supported")
	ErrPubSubDoesNotExist = errors.New("pubsub does not exist")
)

// Attached pubsub providers
var (
	pubsubsLock = new(sync.Mutex)
	pubsubs     = make(map[string]pubsub.SubscribeWriter)
)

// A map of topics and their pubsub providers
var (
	topicPubsubLock = new(sync.Mutex)
	topicPubsub     = make(TopicPubSub)
)

// Relay interface types
type (
	Starter interface {
		Start() error
	}
	Closer interface {
		Close() error
	}
	StartCloser interface {
		Starter
		Closer
	}
)

// Stores a map of topics to pubsub services
type TopicPubSub map[string]map[string]pubsub.SubscribeWriter

// Add a new topic to pubsub map
func (m TopicPubSub) Add(topic, name string, sw pubsub.SubscribeWriter) {
	m[topic] = map[string]pubsub.SubscribeWriter{
		name: sw,
	}
}

// Get a pubsub from the topic pubsub map
func (m TopicPubSub) Get(topic string) (string, pubsub.SubscribeWriter, bool) {
	elm, ok := m[topic]
	if !ok {
		return "", nil, false
	}
	var name string
	var pubsub pubsub.SubscribeWriter
	for name, pubsub = range elm {
		break
	}
	return name, pubsub, true
}

// Delete topic from topic pubsub map
func (m TopicPubSub) Del(topic string) {
	delete(m, topic)
}

// Common client interface
type Client interface {
	Read() ([]byte, error)
	Write([]byte) (int, error)
}

// Add a new pubsub to the relay pubsub map
func AddPubSub(name string, sw pubsub.SubscribeWriter) {
	defer logger.WithField("name", name).Debug("added pubsub to relays")
	// Store pubsub by name
	pubsubsLock.Lock()
	pubsubs[name] = sw
	pubsubsLock.Unlock()
	// Topics to pubsub map
	topicPubsubLock.Lock()
	for _, topic := range sw.Topics() {
		topicPubsub.Add(topic, name, sw)
	}
	topicPubsubLock.Unlock()
}

// Delete a pubsub from the relay pubsub map
func DelPubSub(name string) {
	defer logger.WithField("name", name).Debug("deleted from realy pubsub")
	pubsubsLock.Lock()
	_, ok := pubsubs[name]
	if !ok {
		return
	}
	delete(pubsubs, name)
	pubsubsLock.Unlock()
	// TODO: delete from topics map
	// TODO: stop relay running subscriptions for this pubsub
}

// Each client when they connect spawns a new relay which manages the
// topic subscriptions and bidirectional reading and writting
type Relay struct {
	client        Client
	topics        []string
	subscriptions map[string]pubsub.ReadCloser
	wg            sync.WaitGroup
	closeC        chan bool
}

// Reads messages from the relay client and writes them to the
// appropriate pubsub service
func (relay *Relay) clientReadPump() error {
	logger.Debug("start relay client read loop")
	defer logger.Debug("exit relay client read loop")
	for {
		select {
		case <-relay.closeC:
			return nil
		default:
			break
		}
		// Read messages from client
		b, err := relay.client.Read()
		if err != nil {
			if err == io.EOF { // client has gone away normally
				return nil
			}
			return err
		}
		// Decode msg bytes - each event has a Topic, this is all
		// we need to route the message to appropriate pubsub service
		// and topic
		event := &struct {
			Topic string `json:"topic"`
		}{}
		if err := json.Unmarshal(b, event); err != nil {
			logger.WithError(err).Error("failed to decode relay client message")
			continue // This is not a fatal error, keep going
		}
		// Get the pubsub for this topic
		pubsubsLock.Lock()
		_, sw, ok := topicPubsub.Get(event.Topic)
		pubsubsLock.Unlock()
		if !ok {
			logger.WithFields(logger.F{
				"topic": event.Topic,
				"event": string(b),
			}).Warn("unsupported topic")
			// We don't support this topic
			continue // This is not a fatal error, keep going
		}
		err = sw.Write(pubsub.Message{
			Topic:   event.Topic,
			Payload: b,
		})
		if err != nil {
			logger.WithError(err).Error("failed to write to pubsub service")
		}
	}
}

// Reads messages from the pubsub subscription and writes them to the relay client
func (relay *Relay) subscriptionReadPump(sub pubsub.Reader) error {
	logger.Debug("start relay subscription coroutine")
	defer logger.Debug("exit relay subscription coroutine")
	for {
		select {
		case <-relay.closeC:
			return nil
		default:
			break
		}
		// Read messages from subscription
		msg, err := sub.Read()
		switch err {
		case nil: // no error
			// Write to relay client
			logger.WithFields(logger.F{
				"topic;":  msg.Topic,
				"payload": string(msg.Payload),
			}).Debug("write pubsub message to relay client")
			_, err := relay.client.Write(msg.Payload)
			if err != nil {
				if err == io.ErrClosedPipe {
					return nil
				}
				return err
			}
		case io.EOF: // closed
			return nil
		default: // unexpected error
			return err
		}
	}
}

// Start a subscription with the pubsub for the given topics
func (relay *Relay) startSubscription(name string, topics ...string) error {
	pubsub, ok := pubsubs[name]
	if !ok {
		return ErrPubSubDoesNotExist
	}
	subscription, err := pubsub.Subscribe(topics...)
	if err != nil {
		return err
	}
	relay.subscriptions[name] = subscription
	go func() {
		relay.wg.Add(1)
		if err := relay.subscriptionReadPump(subscription); err != nil {
			logger.WithError(err).Error("unexpected relay pubsub read error")
		}
		relay.wg.Done()
	}()
	return nil
}

// Starts the relays read / write pumps
func (relay *Relay) Start() error {
	// pubsub to topics map
	tm := make(map[string][]string)
	// loop over the topics we want to subscribe too and build a
	// map of pubsubs to topics
	for _, topic := range relay.topics {
		// Get the pubsub name for the topic
		name, _, ok := topicPubsub.Get(topic)
		if !ok {
			return ErrTopicNotSupported
		}
		// Add the topic to the map of pubsubs to topics
		_, ok = tm[name]
		if !ok {
			tm[name] = make([]string, 1)
		}
		tm[name] = append(tm[name], topic)
	}
	// Now loop over our map of pubsubs to topics and create the
	// pubsub subscription
	for name, topics := range tm {
		if err := relay.startSubscription(name, topics...); err != nil {
			return err
		}
	}
	// Start the read pump which reads from the relay client and pumps
	// the events to a pubsub service
	go func() {
		relay.wg.Add(1)
		if err := relay.clientReadPump(); err != nil {
			logger.WithError(err).Error("unexpected relay client read error")
		}
		relay.wg.Done()
	}()
	return nil
}

// Close the relay
func (relay *Relay) Close() error {
	close(relay.closeC)
	// Close relay subscriptions
	for _, subscription := range relay.subscriptions {
		if err := subscription.Close(); err != nil {
			logger.WithError(err).Error("error closing relay subscription")
		}
	}
	relay.wg.Wait()
	return nil
}

// Construct a new Relay
func New(client Client, topics ...string) *Relay {
	return &Relay{
		client:        client,
		topics:        topics,
		subscriptions: make(map[string]pubsub.ReadCloser),
		closeC:        make(chan bool),
	}
}
