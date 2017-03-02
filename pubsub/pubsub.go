// Standard Pub/Sub interfaces

package pubsub

type (
	// A common message type all pub/sub systems give and take
	Message struct {
		Topic   string
		Payload []byte
	}
	// Returns messages for the subscribed topics
	Reader interface {
		Read() (*Message, error)
	}
	// Writes message to topic
	Writer interface {
		Write(Message) error
	}
	// Standard pub/sub close
	Closer interface {
		Close() error
	}
	// Subscribers return read closers
	ReadCloser interface {
		Reader
		Closer
	}
	// A subscriber returns a read closer for receiving messages
	// from a pub/sub subscription
	Subscriber interface {
		Topics() []string
		Subscribe(topics ...string) (ReadCloser, error)
	}
	// A pub/sub system should allow us to subscribe and write
	// messages to the system
	SubscribeWriter interface {
		Subscriber
		Writer
	}
	SubscribeWriteCloser interface {
		SubscribeWriter
		Closer
	}
)
