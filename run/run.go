package run

import (
	"eventrelay/logger"
	"eventrelay/pubsub/redis"
	"eventrelay/relay"
	"eventrelay/websocket"
)

var run = &runner{}

func Run() error {
	if err := run.start(); err != nil {
		return err
	}
	signal := <-UntilQuit()
	logger.WithFields(logger.F{
		"signal": signal.String(),
	}).Debug("received os signal")
	if err := run.stop(); err != nil {
		return err
	}
	return nil
}

type runner struct {
	server *websocket.Server
	redis  *redis.Client
}

func (r *runner) start() error {
	// Redis Pub/Sub
	r.redis = redis.New(redis.NewConfig())
	relay.AddPubSub("redis", r.redis)
	// Start Web Socket Server
	r.server = websocket.NewServer(websocket.NewServerConfig())
	go r.server.ListenAndServe()
	return nil
}

func (r *runner) stop() error {
	if r.redis != nil {
		if err := r.redis.Close(); err != nil {
			logger.WithError(err).Error("error closing redis server")
		}
	}
	if r.server != nil {
		if err := r.server.Close(); err != nil {
			logger.WithError(err).Error("error closing websocket server")
		}
	}
	return nil
}
