package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"

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
	relay.AddPubSub("redis", pubsub, redisConfig.Topics()...)

	srv := websocket.NewServer(websocket.NewServerConfig())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				logger.Debug("http server closed")
				return
			}
			logger.WithError(err).Error("http server listen error")
			return
		}
	}()

	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)

	<-stopChan // wait for SIGINT
	if err := srv.Close(); err != nil {
		fmt.Println(err)
	}
	wg.Wait()

	fmt.Println("exit")
}
