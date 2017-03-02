// Redis Pub/Sub Client

package redis

import (
	"io"
	"sync"

	"eventrelay/logger"
	"eventrelay/pubsub"

	"github.com/rs/xid"
	redis "gopkg.in/redis.v5"
)

// Redis Pub/Sub client
type Client struct {
	Config            Configurer               // Redis client configuration
	connectLock       sync.Mutex               // Connect lock
	redis             *redis.Client            // underlying redis connection
	subscriptionsLock sync.Mutex               // Subscriptions lock
	subscriptions     map[string]*Subscription // Active subscriptions
}

// Connect to redis
func (client *Client) connect() {
	client.connectLock.Lock()
	if client.redis == nil {
		client.redis = redis.NewClient(&redis.Options{
			Addr: client.Config.Host(),
		})
	}
	client.connectLock.Unlock()
}

// Add subscription to client store of subscriptions
func (client *Client) addSubscription(id string, sub *Subscription) {
	client.subscriptionsLock.Lock()
	client.subscriptions[id] = sub
	client.subscriptionsLock.Unlock()
}

// Del subscription from client store of subscriptions
func (client *Client) delSubscription(id string) {
	client.subscriptionsLock.Lock()
	delete(client.subscriptions, id)
	client.subscriptionsLock.Unlock()
}

// Subscribe creates a new subscription for the given
// topics we want to receive messages from, this will return
// the new subscription which implements the standard pubsub.ReadCloser
// interface
func (client *Client) Subscribe(topics ...string) (pubsub.ReadCloser, error) {
	if client.redis == nil {
		client.connect()
	}
	pubsub, err := client.redis.PSubscribe(topics...)
	if err != nil {
		return nil, err
	}
	id := xid.New().String()
	subscription := NewSubscription(id, client, pubsub)
	client.addSubscription(id, subscription)
	return subscription, nil
}

// Writes a message to specifed topic as provided by the pubsub.Message
func (client *Client) Write(message pubsub.Message) error {
	if client.redis == nil {
		client.connect()
	}
	logger.WithFields(logger.F{
		"channel": message.Topic,
		"payload": string(message.Payload),
	}).Debug("publish message to redis channel")
	res := client.redis.Publish(message.Topic, string(message.Payload))
	return res.Err()
}

// Close pub/sub, stops all current subscriptions running
func (client *Client) Close() error {
	// Close subscriptions
	client.subscriptionsLock.Lock()
	var wg sync.WaitGroup
	wg.Add(len(client.subscriptions))
	for _, subscription := range client.subscriptions {
		go func(subscription *Subscription) {
			defer wg.Done()
			if err := subscription.Close(); err != nil {
				logger.WithError(err).Error("error closing redis subscription")
			}
		}(subscription)
	}
	client.subscriptionsLock.Unlock()
	wg.Wait() // Wait for subscriptions to close
	// Close redis client
	if client.redis != nil {
		return client.redis.Close()
	}
	return nil
}

// Constructs a new redis Pub/Sub client
func New(config Configurer) *Client {
	return &Client{
		Config:        config,
		subscriptions: make(map[string]*Subscription),
	}
}

// Redis Pub/Sub Subscription
type Subscription struct {
	id     string        // subscription id
	pubsub *redis.PubSub // Underlying redis subscription
	client *Client       // Redis client
	closeC chan bool     // Close orchestration
}

// Read messages from subscription, returning a pubsub.Message
// On close this method will error with an io.EOF
func (s *Subscription) Read() (*pubsub.Message, error) {
	for {
		// Check close state on each iteration
		select {
		case <-s.closeC:
			return nil, io.EOF
		default:
			break
		}
		// Wait for message from pub/sub subscription - blocking
		msg, err := s.pubsub.ReceiveMessage()
		if err != nil {
			select {
			case <-s.closeC:
				return nil, io.EOF
			default:
				return nil, err
			}
		}
		logger.WithFields(logger.F{
			"topic;":  msg.Channel,
			"payload": msg.Payload,
		}).Debug("recieved message from redis pubsub subscription")
		return &pubsub.Message{
			Topic:   msg.Channel,
			Payload: []byte(msg.Payload),
		}, nil
	}
}

// Close the pubsub subscription
func (s *Subscription) Close() error {
	// remove subscription from client
	s.client.delSubscription(s.id)
	// close subscription
	return s.pubsub.Close()
}

// Constructs a new Subscription
func NewSubscription(id string, client *Client, pubsub *redis.PubSub) *Subscription {
	return &Subscription{
		id:     id,
		client: client,
		pubsub: pubsub,
		closeC: make(chan bool),
	}
}
