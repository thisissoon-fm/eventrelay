package cli

import (
	"fmt"

	"eventrelay/build"

	"github.com/spf13/cobra"
)

// Prints build information
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Prints build information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("OS:", build.OS())
		fmt.Println("Architecture:", build.Architecture())
		fmt.Println("Version:", build.Version())
		fmt.Println("Time:", build.TimeStr())
	},
}

func init() {
	mainCmd.AddCommand(buildCmd)
}
