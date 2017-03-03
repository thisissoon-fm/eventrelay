// Application entry point

package main

import (
	"fmt"

	"eventrelay/cli"
)

func main() {
	if err := cli.Run(); err != nil {
		fmt.Println(err)
	}
}
