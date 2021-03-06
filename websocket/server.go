// Websocket Server
//
// Runs a HTTP server the accepts new incoming connections
// and upgrades them to a persistent websocket connection.
// Each connection is converted into a client and is attached
// to a relay. The server stores each connected client based
// on remote address.

package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"eventrelay/build"
	"eventrelay/logger"

	"github.com/gorilla/websocket"
)

// Upgrade the incoming HTTP connection
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all incoming connections
	},
}

// Websocket Server
type Server struct {
	// Config holds methods for configuring the server
	Config Configurer
	// Clients connected to the server
	clientsLock *sync.Mutex
	clients     map[string]*Client
	clientsC    chan *Client
	// HTTP Server
	srv *http.Server
}

// Healthcheck Handler /__healthcheck___
func (srv *Server) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(&struct {
		Version string `json:"version"`
	}{
		Version: build.Version(),
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

// HTTP handler for handling new incoming connections
func (srv *Server) newConnHandler(w http.ResponseWriter, r *http.Request) {
	// Validate via basic auth
	// TODO: HMAC / JWT?
	user, pass, ok := r.BasicAuth()
	if !ok || user == "" || pass == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if user != srv.Config.Username() || pass != srv.Config.Password() {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	logger.Debug("new websocket client connection")
	// Upgrade HTTP connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WithError(err).Error("error upgrading HTTP connection")
		return
	}
	// Add the client to the server
	topics := strings.Split(r.Header.Get("Topics"), ",")
	client := NewClient(srv, conn, topics...)
	srv.Add(client)
	// Start client consuming messages
	client.Go()
}

// Add client to server clients store
func (s *Server) Add(client *Client) {
	l := logger.WithField("remote", client.RemoteAddr().String())
	l.Debug("add websocket client to server")
	s.clientsLock.Lock()
	s.clients[client.RemoteAddr().String()] = client
	s.clientsLock.Unlock()
}

// Delete a client from the server
func (s *Server) Del(client *Client) {
	l := logger.WithField("remote", client.RemoteAddr().String())
	l.Debug("delete websocket client from server")
	s.clientsLock.Lock()
	delete(s.clients, client.RemoteAddr().String())
	s.clientsLock.Unlock()
}

// Listens for new incoming websocket connections
func (s *Server) ListenAndServe() error {
	// Setup HTTP Server
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(s.newConnHandler))
	mux.Handle("/__healthcheck__", http.HandlerFunc(s.healthcheckHandler))
	srv := &http.Server{
		Addr:    s.Config.Bind(),
		Handler: mux,
	}
	s.srv = srv
	// Start HTTP Server
	return srv.ListenAndServe()
}

// Shuts down http server and closes open connections
func (s *Server) Close() error {
	logger.Debug("close websocket server")
	defer logger.Info("closed websocket server")
	// Close HTTP Server
	if s.srv != nil {
		ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
		if err := s.srv.Shutdown(ctx); err != nil {
			return err
		}
	}
	// Close connected clients
	var wg sync.WaitGroup
	wg.Add(len(s.clients))
	for _, c := range s.clients {
		go func() {
			defer wg.Done()
			if err := c.Close(); err != nil {
				logger.WithError(err).Error("error closing websocket client")
			}
		}()
	}
	wg.Wait()
	return nil
}

// Construct a new websocket server
func NewServer(config Configurer) *Server {
	return &Server{
		Config:      config,
		clientsLock: &sync.Mutex{},
		clients:     make(map[string]*Client),
	}
}
