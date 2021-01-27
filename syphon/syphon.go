package syphon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/choclab-net/tiyo/config"
	"github.com/choclab-net/tiyo/pipeline"
	"github.com/choclab-net/tiyo/server"
	log "github.com/sirupsen/logrus"
)

// Syphon is the command executor embedded inside docker containers
type Syphon struct {
	config   *config.Config
	client   *http.Client
	server   string
	hostname string
	self     string
}

// Create a new syphon executor
func NewSyphon() *Syphon {
	syphon := Syphon{}
	var err error
	syphon.config, err = config.NewConfig()
	if err != nil {
		log.Panic(err)
	}
	syphon.client = &http.Client{}
	syphon.server = syphon.config.FlowServer()
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal("Cannot obtain hostname from system ", err)
	}
	// verification is key...
	syphon.hostname = hostname
	if hostname == "meteor.choclab.net" {
		syphon.hostname = "example-pipeline-test-0"
	}

	var nameSlice []string = strings.Split(syphon.config.AppName, ":")
	syphon.self = strings.Trim(nameSlice[0], "-tiyo")
	return &syphon
}

// send status to flow: one of 'ready'|'busy'
//
// If status is 'ready', a command will be returned when
// one is available. Otherwise, nil is returned
func (syphon *Syphon) register(status string) *server.QueueItem {
	content := make(map[string]string)
	content["pod"] = syphon.hostname
	content["container"] = syphon.config.AppName
	content["status"] = status
	data, _ := json.Marshal(content)

	request, err := http.NewRequest(
		http.MethodPost,
		syphon.server+"/api/v1/register",
		bytes.NewBuffer(data))

	if err != nil {
		log.Error(err)
		return nil
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Connection", "close")
	request.Close = true

	response, err := syphon.client.Do(request)
	if err != nil {
		log.Error(err)
		return nil
	}

	log.Info("Recieved response with status code ", response.StatusCode, " for status ", status)
	var message string = ""
	if response.StatusCode == http.StatusAccepted || response.StatusCode == http.StatusNoContent {
		// Flow has accepted our update but has no command to return
		message = "No command returned. Sleeping for 10 seconds before checking again"
		if status == "Busy" {
			// dont log if we're only updating status
			message = ""
		}
	} else if response.StatusCode == 404 {
		// The queue has not been loaded
		message = "No queue or no queue active - sleeping for 10 seconds before checking again"
	}

	if message != "" || status == "Busy" {
		response.Body.Close()
		if message != "" {
			log.Info(message)
		}
		return nil
	}

	var body []byte
	defer response.Body.Close()
	body, err = ioutil.ReadAll(response.Body)
	if err != nil {
		log.Error(err)
		return nil
	}

	command := server.QueueItem{}
	result := server.Result{
		Message: &command,
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Error(err)
		return nil
	}
	return &command
}

func (syphon *Syphon) requeue(queueItem *server.QueueItem) {}

func (syphon *Syphon) log(code int, command *pipeline.Command) {}

func (syphon *Syphon) execute(queueItem *server.QueueItem) {
	command := &queueItem.Command
	var baseDir string = filepath.Join(syphon.config.SequenceBaseDir, queueItem.PipelineFolder, queueItem.SubFolder)
	log.Info("Recieved filename ", filepath.Join(baseDir, queueItem.Filename), " with command ", command)

	var exitCode int
	if exitCode = command.Execute(baseDir, syphon.self, queueItem.Filename, queueItem.Event); exitCode != 0 {
		// if exitcode is not 0, add the command back to the queue
		// requeue should send logs back with the command
		syphon.requeue(queueItem)
	} else {
		// write command logs back to the log bucket
		syphon.log(exitCode, command)
	}

	// if no end-time, command timed out.
	if command.EndTime != 0 {
		h, m, s := time.Unix(0, command.EndTime-command.StartTime).Clock()
		log.Info(fmt.Sprintf("Command completed in %d:%d:%d", h, m, s))
	}
}

// Syphon will not have an initialiser.
func (syphon *Syphon) Init() {}

// Run the syphon command executor
//
// This is the main entry point for the syphon command and is executed
// from command.Command package.
//
// When triggered, syphon will register against the flow server once every
// 10 seconds. If the registration returns a command, syphon will then
// register itself as busy, execute the returned command and return the
// output of the command back to the flow server on completion.
func (syphon *Syphon) Run() int {
	log.Info("Starting tiyo syphon - ", syphon.self)
	sigc := make(chan os.Signal, 1)
	done := make(chan bool)
	signal.Notify(sigc, os.Interrupt)
	go func() {
		for range sigc {
			log.Info("Shutting down listener")
			done <- true
		}
	}()

	go func() {
		for {
			log.Info("Polling ", syphon.config.Flow.Host, ":", syphon.config.Flow.Port)
			if command := syphon.register("Ready"); command != nil {
				log.Info("Registering busy and executing command")
				syphon.register("Busy")
				syphon.execute(command)
			}

			// Check for a new command every 10 seconds
			time.Sleep(10 * time.Second)
		}
	}()
	<-done
	return 0
}
