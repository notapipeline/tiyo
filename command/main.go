package command

import (
	"fmt"
	"os"
	"path/filepath"

	flow "github.com/choclab-net/tiyo/flow"
	server "github.com/choclab-net/tiyo/server"
	watch "github.com/choclab-net/tiyo/watch"
	log "github.com/sirupsen/logrus"
)

const VERSION string = "v0.0.1a"

type Command interface {
	Init()
	Run() int
}

var acceptedCommands = []string{
	"assemble",
	"fill",
	"flow",
	"syphon",
	"help",
	"version",
}

func SetupLog() {
	var level string = os.Getenv("TIYO_LOG")
	if level == "" {
		level = "info"
	}
	switch level {
	case "trace":
		log.SetLevel(log.TraceLevel)
		log.SetReportCaller(true)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	}
}

func Usage() {
	fmt.Printf("USAGE: %s [COMMAND] [FLAGS]:\n", filepath.Base(os.Args[0]))
	for _, command := range acceptedCommands {
		fmt.Printf("    - %s\n", command)
	}
	fmt.Printf("Run `%s COMMAND -h` to see usage for that given command\n", filepath.Base(os.Args[0]))

}

func Run(args []string) int {
	SetupLog()
	var command string = "help"
	if len(args) != 0 {
		for _, c := range acceptedCommands {
			if c == args[0] {
				command = c
				break
			}
		}
	}

	if command == "" {
		fmt.Printf("Command '%s' not valid for %s\n", args[0], filepath.Base(os.Args[0]))
		Usage()
		return 1
	}

	var instance Command
	switch command {
	case "help":
		Usage()
		return 0
	case "version":
		fmt.Printf("%s version %s\n", filepath.Base(os.Args[0]), VERSION)
		return 0
	case "assemble":
		instance = server.NewServer()
	case "fill":
		instance = watch.NewWatch()
	case "flow":
		instance = flow.NewFlow()
		break
	case "syphon":
		break
	}

	instance.Init()
	code := instance.Run()
	return code
}
