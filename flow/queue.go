// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package flow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"

	"github.com/notapipeline/tiyo/config"
	"github.com/notapipeline/tiyo/pipeline"

	"github.com/notapipeline/tiyo/server"
	log "github.com/sirupsen/logrus"
)

// The queue structure for handling event scheduling into syphon executors

// MAXQUEUE defines how many events should be written into the queue every n seconds
const MAXQUEUE = 100000

// Queue : Defines the structure of the queue item
type Queue struct {

	// The queue bucket to write to
	QueueBucket string

	// The files bucket to write to
	FilesBucket string

	// A bucket to store pod information in
	PodBucket string

	// A bucket to store non-filesystem events in
	EventsBucket string

	// The pipeline bucket to assign against
	PipelineBucket string

	// Configuration for the queue
	Config *config.Config

	// The pipeline used by the queue
	Pipeline *pipeline.Pipeline

	// HTTP client
	Client *http.Client

	// Is the queue stopped or not
	Stopped bool
}

// NewQueue : Create a new queue instance
func NewQueue(config *config.Config, pipeline *pipeline.Pipeline, bucket string) *Queue {
	log.Info("Initialising Queue system")
	queue := Queue{
		QueueBucket:    "queue",
		FilesBucket:    "files",
		PodBucket:      "pods",
		EventsBucket:   "events",
		PipelineBucket: bucket,
		Config:         config,
		Pipeline:       pipeline,
		Client: &http.Client{
			Timeout: config.TIMEOUT,
		},
		Stopped: true,
	}
	queue.createBuckets()
	return &queue
}

// TODO: Split this so the api method sits in API and the
// queue management is here.

// Register : Registers a container into the queue executors
func (queue *Queue) Register(request map[string]interface{}) *server.Result {
	var key string = request["container"].(string) + ":" + request["pod"].(string)
	log.Debug(queue.PodBucket)
	data := queue.jsonBody(queue.PodBucket, key, request["status"].(string))
	result := queue.put(data)
	if request["status"] == "Ready" {
		var (
			code    int
			message *server.QueueItem = nil
		)
		if !queue.Stopped {
			code, message = queue.GetQueueItem(request["container"].(string), request["pod"].(string))
			result.Code = code
		}
		if message != nil {
			result.Message = *message
		} else {
			result.Message = ""
		}
	}
	return result
}

// Stop : stops the current queue
func (queue *Queue) Stop() {
	queue.Stopped = true
}

// Start : starts the current queue as a background process
func (queue *Queue) Start() {
	go queue.perpetual()
}

// GetQueueItem : Get a command to execute
func (queue *Queue) GetQueueItem(container string, pod string) (int, *server.QueueItem) {
	serverAddress := queue.Config.AssembleServer()

	var key string = container + ":" + pod
	req, err := http.NewRequest(http.MethodGet,
		serverAddress+"/api/v1/popqueue/"+queue.Pipeline.Name+"/"+key, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Accept", "application/json")
	code, body := queue.makeRequest(req)

	item := server.QueueItem{}
	err = json.Unmarshal(body, &item)
	if err != nil {
		log.Error(err)
		return code, nil
	}

	parentEnv := make([]string, 0)
	upstreamContainer := queue.Pipeline.ContainerFromCommandID(item.Command.ID)
	if upstreamContainer != nil {
		// pipeline first, then override with kubernetes set type environment
		parentEnv = append(queue.Pipeline.Environment, upstreamContainer.Environment...)
		item.Command.Environment = append(parentEnv, item.Command.Environment...)
	} else {
		item.Command.Environment = append(queue.Pipeline.Environment, item.Command.Environment...)
	}
	return code, &item
}

// put : Put data into the bolt store
func (queue *Queue) put(request []byte) *server.Result {
	result := server.NewResult()
	result.Code = 204
	result.Result = "No content"
	result.Message = ""

	serverAddress := queue.Config.AssembleServer()
	req, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("%s/api/v1/bucket", serverAddress),
		bytes.NewBuffer(request))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Connection", "close")
	req.Close = true

	code, _ := queue.makeRequest(req)
	result.Code = code
	return result
}

// makeRequest : Retries a HTTP request up to 5 times
func (queue *Queue) makeRequest(request *http.Request) (int, []byte) {
	var (
		maxRetries int = 5
		retries    int = maxRetries
		err        error
		response   *http.Response
		body       []byte
	)
	for retries > 0 {
		response, err = queue.Client.Do(request)
		if err == nil {
			break
		}
		retries--
	}
	if err != nil {
		log.Error(err)
		return 500, nil
	}

	defer response.Body.Close()
	body, err = ioutil.ReadAll(response.Body)
	if err != nil {
		log.Error(err)
		return response.StatusCode, nil
	}
	return response.StatusCode, body
}

// jsonBody : construct the jsonBody for writing information into an event bucket
func (queue *Queue) jsonBody(bucket string, key string, value string) []byte {
	bucket = filepath.Base(bucket)
	values := map[string]string{
		"bucket": bucket,
		"child":  queue.PipelineBucket,
		"key":    key,
		"value":  value,
	}

	jsonValue, _ := json.Marshal(values)
	return jsonValue
}

// createBuckets : Create any missing queue buckets for this pipeline
func (queue *Queue) createBuckets() {
	buckets := []string{queue.PodBucket, queue.EventsBucket, queue.FilesBucket, queue.QueueBucket}
	for _, bucket := range buckets {
		content := make(map[string]string)
		content["bucket"] = bucket
		content["child"] = queue.PipelineBucket
		body, err := json.Marshal(content)
		if err != nil {
			log.Error("Failed to create bucket ", bucket, "/", queue.PipelineBucket, " : ", err)
		}
		serverAddress := queue.Config.AssembleServer()
		request, _ := http.NewRequest(http.MethodPost, serverAddress+"/api/v1/bucket", bytes.NewBuffer(body))
		request.Header.Set("Content-Type", "application/json; charset=utf-8")
		request.Header.Set("Connection", "close")
		request.Close = true
		queue.makeRequest(request)
	}
}

// perpetual : run the queue every n seconds to move events from the
// event buckets to the queue bucket
func (queue *Queue) perpetual() {
	log.Info("Setting up perpetual queue for ", queue.Pipeline.Name)
	var first bool = true
	queue.Stopped = false
	for {
		if queue.Stopped {
			break
		}
		if !first {
			time.Sleep(10 * time.Second)
		}

		first = false
		log.Info("Updating queue for ", queue.Pipeline.Name)
		content := make(map[string]interface{})
		content["pipeline"] = queue.Pipeline.Name
		content["maxitems"] = MAXQUEUE
		data, _ := json.Marshal(content)

		serverAddress := queue.Config.AssembleServer()
		request, err := http.NewRequest(
			http.MethodPost,
			serverAddress+"/api/v1/perpetualqueue",
			bytes.NewBuffer(data))

		if err != nil {
			log.Error(err)
			continue
		}
		request.Header.Set("Content-Type", "application/json; charset=utf-8")
		request.Header.Set("Connection", "close")
		request.Close = true

		response, err := queue.Client.Do(request)
		if err != nil {
			log.Error(err)
			continue
		}

		if response.StatusCode != http.StatusAccepted {
			log.Error("Error during processing queue ", response)
			continue
		}
		response.Body.Close()
	}
	log.Info("Queue terminated ", queue.Pipeline.Name)
	queue.Stopped = true
}
