package cli

import (
	"eventrelay/config"
	"eventrelay/logger"
	"eventrelay/run"

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
		if err := run.Run(); err != nil {
			logger.WithError(err).Error("error running application")
		}
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
