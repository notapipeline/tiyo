// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package fill acts as a sub-command, reading inotify events and
// forwarding them to the boldb backing the assemble server.
//
// By default, the fill command is designed to listen for only
// open, close and delete events tying it to the Linux subsystem.
//
// Whilst Windows/Mac can generate similar events, these are not
// handled by the `tiyo fill` application and as a result, this
// section of the application is not designed for those platforms.
package fill

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"

	"github.com/notapipeline/tiyo/pkg/config"
	"github.com/notapipeline/tiyo/pkg/pipeline"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

// Fill : Primary structure of the fill command
type Fill struct {

	// The filler object managed by this instance of the fill command
	Filler *Filler

	// Configuration of the fill command
	Config *config.Config

	// The pipeline being handled by this command
	Pipeline *pipeline.Pipeline

	// The name of the pipeline to load
	Name string

	// Command flags
	Flags *flag.FlagSet
}

// NewFill Create a new fill command
func NewFill() *Fill {
	log.Info("Starting new fill executor")
	fill := Fill{}
	return &fill
}

// Init the command according to the flags and environment variables provided
func (fill *Fill) Init() {
	fill.Name = os.Getenv("TIYO_PIPELINE")
	description := "The name of the pipeline to use"
	fill.Flags = flag.NewFlagSet("fill", flag.ExitOnError)
	fill.Flags.StringVar(&fill.Name, "p", fill.Name, description)
	fill.Flags.Parse(os.Args[2:])
	if fill.Name == "" {
		fill.Flags.Usage()
		os.Exit(1)
	}
}

// Listen for file events on all watch paths defined in the pipeline
func (fill *Fill) fill() {
	var matchers []pipeline.Matcher = fill.Pipeline.WatchItems()
	channels := make([]chan notify.EventInfo, len(matchers))

	for i := 0; i < len(matchers); i++ {
		channels[i] = make(chan notify.EventInfo, 1)
		var path = filepath.Join(
			fill.Config.SequenceBaseDir, fill.Config.Kubernetes.Volume,
			fill.Pipeline.BucketName, matchers[i].Source)

		log.Info("Creating channel for ", path)
		os.MkdirAll(path, os.ModePerm)

		// each event stream is executed in its own goroutine to isolate from other watchers
		go func(directory string, match *regexp.Regexp, channel chan notify.EventInfo) {
			log.Info("Start listening for ", directory, " with match ", match)
			if err := notify.Watch(directory, channel, notify.InOpen, notify.InCloseWrite, notify.Remove); err != nil {
				log.Fatal(err)
				return
			}
			for {
				eventInfo := <-channel
				// only store events which match the pattern given
				if !match.MatchString(filepath.Base(eventInfo.Path())) {
					return
				}

				fi, err := os.Stat(eventInfo.Path())
				if err == nil {
					switch mode := fi.Mode(); {
					case mode.IsDir():
						log.Warn("Skipping directory ", eventInfo.Path())
						return
					}
				}
				var (
					filename string = filepath.Base(eventInfo.Path())
					dirname  string = filepath.Base(filepath.Dir(eventInfo.Path()))
				)
				matches := match.FindStringSubmatch(filename)
				if len(matches) > 1 {
					filename = matches[1] // should be widest possible grouping match
				}
				fill.Filler.Add(fill.Pipeline.BucketName, dirname, filename, eventInfo.Event())
			}
		}(path, matchers[i].Pattern, channels[i])
	}

}

// Run the fill application to listen for monitored file events
func (fill *Fill) Run() int {
	sigc := make(chan os.Signal, 1)
	done := make(chan bool)
	signal.Notify(sigc, os.Interrupt)
	go func() {
		for range sigc {
			log.Info("Shutting down listener")
			done <- true
		}
	}()

	var (
		err error
	)
	fill.Config, err = config.NewConfig()
	if err != nil {
		log.Error("Error loading config file: ", err)
		return 1
	}

	fill.Pipeline, err = pipeline.GetPipeline(fill.Config, fill.Name)
	if err != nil {
		log.Error("Error loading pipeline ", fill.Name, " - does the pipeline exist?", err)
		return 1
	}

	fill.Filler = NewFiller(fill.Config)
	fill.fill()
	<-done

	return 0
}
