package cli

import (
	"os"
	"os/signal"

	"eventrelay/config"
	"eventrelay/logger"
	"eventrelay/pubsub/redis"
	"eventrelay/relay"
	"eventrelay/websocket"

	"github.com/spf13/cobra"
)

// Main CLI command flag vars
var (
	mainCmdConfigFlag string // Optional config path flag
)

// Main CLI Entry Point
var mainCmd = &cobra.Command{
	Use:   "eventrelay",
	Short: "Run the event relay",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Load configuration
		if err := config.Load(mainCmdConfigFlag); err != nil {
			logger.WithError(err).Warn("failed to load configuration")
		}
		// Setup Global Logger
		logger.SetGlobalLogger(logger.New(logger.NewConfig()))
	},
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("start")
		// Redis pub/sub
		pubsub := redis.New(redis.NewConfig())
		defer pubsub.Close()
		relay.AddPubSub("redis", pubsub)
		// Start websocket server
		srv := websocket.NewServer(websocket.NewServerConfig())
		go srv.ListenAndServe()
		defer srv.Close()
		// Wait for exit sygna;s
		stopChan := make(chan os.Signal)
		signal.Notify(stopChan, os.Interrupt)
		<-stopChan // wait for SIGINT
		logger.Info("exit")
	},
}

// Add main command cli flags
func init() {
	mainCmd.PersistentFlags().StringVarP(
		&mainCmdConfigFlag,
		"config",
		"c",
		"",
		"Absolute path to configuration file")
}

// Run the CLI
func Run() error {
	return mainCmd.Execute()
}
