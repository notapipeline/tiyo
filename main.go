package main

import (
	"os"

	"github.com/mproffitt/tiyo/command"
)

func main() {
	os.Exit(command.Run(os.Args[1:]))
}
