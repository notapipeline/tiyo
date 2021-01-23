package syphon

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/choclab-net/tiyo/config"
	"github.com/choclab-net/tiyo/server"
	log "github.com/sirupsen/logrus"
)

type Syphon struct {
	Config   *config.Config
	client   *http.Client
	server   string
	hostname string
}

func NewSyphon() *Syphon {
	syphon := Syphon{}
	var err error
	syphon.Config, err = config.NewConfig()
	if err != nil {
		log.Panic(err)
	}
	syphon.client = &http.Client{}

	syphon.server = syphon.Config.FlowServer()
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal("Cannot obtain hostname from system ", err)
	}
	// verification is key...
	syphon.hostname = hostname
	if hostname == "meteor.choclab.net" {
		syphon.hostname = "example-pipeline-test-0"
	}
	return &syphon
}

// send status to flow: one of 'ready'|'busy'
//
// If status is 'ready', a command will be returned when
// one is available. Otherwise, nil is returned
func (syphon *Syphon) register(status string) *server.QueueItem {
	content := make(map[string]string)
	content["pod"] = syphon.hostname
	content["container"] = syphon.Config.AppName
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

	var message string = ""
	if response.StatusCode == http.StatusAccepted || response.StatusCode == http.StatusNoContent {
		// Flow has accepted our update but has no command to return
		message = "No command returned. Sleeping for 10 seconds before checking again"
	} else if response.StatusCode == 404 {
		// The queue has not been loaded
		message = "No queue or no queue active - sleeping for 10 seconds before checking again"
	}

	if message != "" {
		log.Info(message)
		response.Body.Close()
		return nil
	}

	var body []byte
	defer response.Body.Close()
	body, err = ioutil.ReadAll(response.Body)
	if err != nil {
		log.Error(err)
		return nil
	}

	log.Debug(string(body))
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

func (syphon *Syphon) execute(queueItem *server.QueueItem) {
	command := queueItem.Command
	log.Debug("Recieved filename ", queueItem.Filename, " with command ", command)
	command.Execute(queueItem.PipelineFolder, queueItem.Filename, queueItem.Event)
}

// Syphon will not have an initialiser.
func (syphon *Syphon) Init() {}

// Run the syphon command executor
func (syphon *Syphon) Run() int {
	log.Info("Starting tiyo syphon")
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
			log.Info("Polling ", syphon.Config.Flow.Host, ":", syphon.Config.Flow.Port)
			if command := syphon.register("Ready"); command != nil {
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
