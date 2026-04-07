package main

import (
	"os"

	"github.com/anastanveer653/envy/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}




