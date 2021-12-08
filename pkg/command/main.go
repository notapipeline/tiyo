// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package command serves as the main entry point to the tiyo application
// and its relevant primary sub-commands `assemble`, `flow`, `fill` and `syphon`
package command

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/notapipeline/tiyo/pkg/config"
	"github.com/notapipeline/tiyo/pkg/fill"
	"github.com/notapipeline/tiyo/pkg/flow"
	"github.com/notapipeline/tiyo/pkg/server"
	"github.com/notapipeline/tiyo/pkg/syphon"
	log "github.com/sirupsen/logrus"
)

// VERSION : The applications current version number
const VERSION string = "v0.0.1a"

// Command : define the main interface for sub-commands
//
// Any primary subcommand executable by this wrapper
// needs to implement this interface.
type Command interface {
	Init()
	Run() int
}

// static list of subcommands accepted by tiyo
var acceptedCommands = []string{
	"assemble",
	"fill",
	"flow",
	"syphon",
	"help",
	"version",
}

// SetupLog : configure the primary application logging system
//
// By default, the logging level is info but this can
// be controlled by the environment variable TIYO_LOG
// if a finer level of control is required.
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
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	}
}

// Usage : print usage information about the main application
func Usage() {
	fmt.Printf("USAGE: %s [COMMAND] [FLAGS]:\n", filepath.Base(os.Args[0]))
	for _, command := range acceptedCommands {
		fmt.Printf("    - %s\n", command)
	}
	fmt.Printf("Run `%s COMMAND -h` to see usage for that given command\n", filepath.Base(os.Args[0]))

}

// Run : Main entry for the primary wrapper
func Run(args []string) int {
	SetupLog()
	var (
		command  string = "help"
		instance Command
		code     int = 1
	)
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

	config.Designate = command
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
		instance = fill.NewFill()
	case "flow":
		instance = flow.NewFlow()
		break
	case "syphon":
		instance = syphon.NewSyphon()
		break
	}

	if instance != nil {
		instance.Init()
		code = instance.Run()
	}
	return code
}
