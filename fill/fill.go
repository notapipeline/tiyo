package fill

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"

	"github.com/choclab-net/tiyo/config"
	"github.com/choclab-net/tiyo/pipeline"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

type Fill struct {
	Filler         *Filler
	Config         *config.Config
	Pipeline       *pipeline.Pipeline
	Name           string
	Flags          *flag.FlagSet
	PipelineBucket string
}

func NewFill() *Fill {
	log.Info("Starting new fill executor")
	filler := Fill{}
	return &filler
}

// pipeline must previously exist
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

func (fill *Fill) fill() {
	var matchers []pipeline.Matcher = fill.Pipeline.WatchItems()
	channels := make([]chan notify.EventInfo, len(matchers))

	for i := 0; i < len(matchers); i++ {
		channels[i] = make(chan notify.EventInfo, 1)
		var path = filepath.Join(fill.Config.SequenceBaseDir, fill.Pipeline.BucketName, matchers[i].Source)
		log.Info("Creating channel for ", path)
		os.MkdirAll(path, os.ModePerm)

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
				var filename string = filepath.Base(eventInfo.Path())
				matches := match.FindStringSubmatch(filename)
				if len(matches) > 1 {
					filename = matches[1] // should be widest possible grouping match
				}
				fill.Filler.Add(fill.Pipeline.BucketName, eventInfo.Path(), eventInfo.Event())
			}
		}(path, matchers[i].Pattern, channels[i])
	}

}

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
