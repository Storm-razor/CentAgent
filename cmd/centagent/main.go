package main

import (
	"github.com/wwwzy/CentAgent/internal/cli"
	"os"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
