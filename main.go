package main

import (
	"fmt"
	"os"
	"os/signal"

	"eventrelay/config"
	"eventrelay/logger"
	"eventrelay/pubsub/redis"
	"eventrelay/relay"
	"eventrelay/websocket"
)

// TODO: move to CLI / Run packages

func main() {
	fmt.Println("start")
	if err := config.Load(""); err != nil {
		logger.WithError(err).Warn("failed to load configuration")
	}

	redisConfig := redis.NewConfig()
	pubsub := redis.New(redisConfig)
	defer pubsub.Close()
	relay.AddPubSub("redis", pubsub)

	srv := websocket.NewServer(websocket.NewServerConfig())
	go srv.ListenAndServe()
	defer srv.Close()

	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)
	<-stopChan // wait for SIGINT
	fmt.Println("exit")
}
