package main

import (
	"os"

	"github.com/choclab-net/tiyo/command"
)

func main() {
	os.Exit(command.Run(os.Args[1:]))
}
