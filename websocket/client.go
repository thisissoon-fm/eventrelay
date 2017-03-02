// Websocket Client
//
// A client represents a single websocket connection. Each client
// handles maintaining the connection, reading and writting messages with
// the remote connection. The client is attached to a relay at time of
// connection creation by the server, however the client itself, when
// it closes handles the removal of the client from the relay.

package websocket

import (
	"io"
	"net"
	"sync"
	"time"

	"eventrelay/logger"
	"eventrelay/relay"

	"github.com/gorilla/websocket"
)

var (
	writeWait      = 10 * time.Second    // Time allowed to write a message to the peer.
	pingPeriod     = (pongWait * 9) / 10 // Send pings to peer with this period. Must be less than pongWait.
	pongWait       = 20 * time.Second    // Time allowed to read the next pong message from the peer.
	maxMessageSize = int64(512 * 1024)   // Maximum message size allowed from peer. (512kb)
)

// Websocket client connection
type Conn interface {
	// Connection info
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	// Read
	SetReadLimit(int64)
	SetReadDeadline(time.Time) error
	ReadMessage() (messageType int, p []byte, err error)
	// Write
	SetWriteDeadline(time.Time) error
	WriteControl(messageType int, data []byte, deadline time.Time) error
	WriteMessage(messageType int, data []byte) error
	// Pong Handler
	SetPongHandler(func(string) error)
	// Close
	Close() error
}

// Websocket server client
type Client struct {
	conn   Conn     // Underlying websocket connection
	srv    *Server  // Underlying websocket server
	topics []string // Topics the client subscribes too
	// I/O channles
	writeC chan []byte
	readC  chan []byte
	// Relay
	relay relay.StartCloser
	// Close orchestration
	wg             *sync.WaitGroup // Wait for goroutines to exit
	closeLock      *sync.Mutex     // Lock to prevent multiple close calls
	closedByRemote bool            // Client closed by the remote
	closed         bool            // Closed state
	closeC         chan bool       // Close channel breaks goroutines
}

// Configures the websocket connection
func (client *Client) configureConn() {
	client.conn.SetReadLimit(maxMessageSize)
	client.conn.SetPongHandler(client.pong)
}

// pong handler
func (client *Client) pong(data string) error {
	logger.WithField("data", data).Debug("pong from websocket remote client")
	return nil
}

// Starts the read and write goroutines
func (client *Client) Go() {
	// Configure connection
	client.configureConn()
	// Start a new relay to consume and write to the client
	client.relay = relay.New(client, client.topics...)
	if err := client.relay.Start(); err != nil {
		logger.WithError(err).Error("error spawning websocket client relay")
		// In the event of the relay failing to spawn we should
		// immediately close the websocket connection since nothing
		// will be reading from it which will cause blocking
		if err := client.closeConn(); err != nil {
			logger.WithError(err).Error("error closing after relay error")
		}
	}
	// Start Write Pump
	go func() {
		client.wg.Add(1)
		defer client.wg.Done()
		if err := client.writePump(); err != nil {
			if err := client.closeConn(); err != nil {
				logger.WithError(err).Error("error closing after write error")
			}
		}
	}()
	// Start Read Pump
	go func() {
		client.wg.Add(1)
		defer client.wg.Done()
		if err := client.readPump(); err != nil {
			if err := client.closeConn(); err != nil {
				logger.WithError(err).Error("error closing after read error")
			}
		}
	}()
}

// Returns the clients local address
func (client *Client) LocalAddr() net.Addr {
	return client.conn.LocalAddr()
}

// Returns the clients remote address
func (client *Client) RemoteAddr() net.Addr {
	return client.conn.RemoteAddr()
}

// Write a message to the client
func (client *Client) Write(msg []byte) (int, error) {
	select {
	case <-client.closeC:
		return 0, io.ErrClosedPipe
	default:
		break
	}
	client.writeC <- msg
	return len(msg), nil
}

// Write a message to the websocket connection
func (client *Client) write(messageType int, message []byte) error {
	var err error
	switch messageType {
	case websocket.PingMessage:
		deadline := time.Now().Add(writeWait)
		err = client.conn.WriteControl(websocket.PingMessage, []byte{}, deadline)
	case websocket.TextMessage:
		err = client.conn.WriteMessage(websocket.TextMessage, message)
	}
	return err
}

// Writes messages to the websocket connection
func (client *Client) writePump() error {
	l := logger.WithFields(logger.F{
		"remote": client.RemoteAddr().String(),
		"local":  client.LocalAddr().String(),
	})
	l.Debug("start client write pump")
	defer l.Debug("exit client write pump")
	for {
		select {
		case <-client.closeC:
			return nil
		case <-time.After(pingPeriod):
			l.Debug("ping client")
			if err := client.write(websocket.PingMessage, nil); err != nil {
				l.WithError(err).Error("ping client error")
				return err
			}
		case msg := <-client.writeC:
			l.WithFields(logger.F{
				"message": string(msg),
			}).Debug("write message to websocket client")
			if err := client.write(websocket.TextMessage, msg); err != nil {
				l.WithError(err).Error("write client error")
				return err
			}
		}
	}
}

// Read messages from the client
func (client *Client) Read() ([]byte, error) {
	select {
	case <-client.closeC:
		return nil, io.EOF
	case msg := <-client.readC:
		return msg, nil
	}
}

// Read messages from the websocket connection, runs as a goroutine, messages are
// placed onto the read channel
func (client *Client) readPump() error {
	logger := logger.WithFields(logger.F{
		"remote": client.RemoteAddr().String(),
		"local":  client.LocalAddr().String(),
	})
	logger.Debug("start client read pump")
	defer logger.Debug("exit client read pump")
	for {
		typ, msg, err := client.conn.ReadMessage()
		if err != nil {
			select {
			case <-client.closeC:
				break // We triggered the close so it's fine :)
			default:
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					logger.WithError(err).Error("unexpected websocket close")
				}
				// This will error if the client connection closed for what ever reason
				client.closedByRemote = true
				logger.Debug("client connection closed by remote")
				return err
			}
			return nil
		}
		// Add the message to the channel
		if typ == websocket.TextMessage {
			client.readC <- msg
		}
	}
}

// Closes the open client websocket client connection
func (client *Client) closeConn() error {
	client.closeLock.Lock()
	if !client.closed {
		logger.Debug("close client connection")
		defer logger.Info("closed client connection")
		// Close closeC - breaks the write pump / read pumps
		close(client.closeC)
		// Remove the client from the server
		if client.srv != nil {
			client.srv.Del(client)
		}
		// Close the client connection if not already closed by the remote
		if !client.closedByRemote {
			msg := websocket.FormatCloseMessage(
				websocket.CloseNormalClosure,
				"close connection")
			if err := client.conn.WriteMessage(websocket.CloseMessage, msg); err != nil {
				logger.WithError(err).Error("error writting websocket close messsage")
			}
		}
		// Stop relay
		if client.relay != nil {
			if err := client.relay.Close(); err != nil {
				logger.WithError(err).Error("error closing relay")
			}
		}
		// Close websocket connection
		if err := client.conn.Close(); err != nil {
			logger.WithError(err).Error("error closing relay")
		}
		client.closed = true
	}
	client.closeLock.Unlock()
	return nil
}

// Close connection and wait for goroutines to exit
func (client *Client) Close() error {
	defer logger.Info("closed client")
	if err := client.closeConn(); err != nil {
		return err
	}
	client.wg.Wait()
	return nil
}

// Constructs a new websocket server client
func NewClient(srv *Server, conn Conn, topics ...string) *Client {
	return &Client{
		conn:   conn,   // Underlying websocket connection
		srv:    srv,    // Underlying websocket server
		topics: topics, // Topics the client subscribes too
		// I/O channles
		writeC: make(chan []byte, 1),
		readC:  make(chan []byte, 1),
		// Close orchestration
		wg:        &sync.WaitGroup{}, // Wait for goroutines to exit
		closeLock: &sync.Mutex{},     // Lock to prevent multiple close calls
		closeC:    make(chan bool),   // Close channel breaks goroutines
	}
}
