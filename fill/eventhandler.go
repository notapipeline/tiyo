// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package fill

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/notapipeline/tiyo/config"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

// MAXCLIENTS : The maximum number of http clients created by fill application
const MAXCLIENTS int = 100

// A channel to write requests into to be handled by the client goroutine
var requests chan *http.Request = make(chan *http.Request)

// FilledFileEvent : A file event to be sent to AssembleServer
type FilledFileEvent struct {

	// The name of the file being actioned
	Filename string

	// The bucket to write into
	Bucket string

	// Is this a file open event
	Opened bool

	// Is this a file close event
	Closed bool

	// Is this a file deleted event
	Deleted bool

	// the configuration object for the fill command
	// Can be a reduced config containing only assemble and
	// the sequence base directory
	Config *config.Config
}

// NewFillEvent : Create a new fill event stream
func NewFillEvent(config *config.Config, bucket string, filename string) *FilledFileEvent {
	event := FilledFileEvent{
		Bucket:   bucket,
		Filename: filename,
		Opened:   false,
		Closed:   false,
		Deleted:  false,
		Config:   config,
	}
	return &event
}

// State : Set the state of the event based on the inotify event
//
// Only monitoring:
//   - InOpen
//   - InCloseWrite
//   - Remove
func (event *FilledFileEvent) State(notification notify.Event) *FilledFileEvent {
	switch notification {
	case notify.InOpen:
		if event.Opened {
			return nil
		}
		event.Opened = true
		event.Deleted = false
	case notify.InCloseWrite:
		if event.Closed {
			return nil
		}
		event.Closed = true
	case notify.Remove:
		if event.Deleted {
			return nil
		}
		event.Deleted = true
		event.Opened = false
		event.Closed = false
	default:
		return nil
	}
	return event
}

// Store : Store the event in the BoltDB
func (event *FilledFileEvent) Store() {
	server := event.Config.AssembleServer()
	data := event.JSONBody(event.Bucket, event.Filename)
	request, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("%s/api/v1/bucket", server),
		bytes.NewBuffer(data))
	if err != nil {
		log.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Connection", "close")
	request.Close = true
	requests <- request
}

// Delete : Deletes an item from the boltdb - triggered on file deleted
func (event *FilledFileEvent) Delete() {
	server := event.Config.AssembleServer()
	request, err := http.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("%s/api/v1/bucket/%s/%s", server, event.Bucket, event.Filename), nil)
	if err != nil {
		log.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Connection", "close")
	request.Close = true
	requests <- request
}

// JSONBody : Construct the JSON object to send as part of the request
func (event *FilledFileEvent) JSONBody(bucket string, key string) []byte {
	bucket = filepath.Base(bucket)
	values := map[string]string{
		"bucket": "files",
		"child":  bucket,
		"key":    key,
		"value":  "",
	}
	value := make(map[string]interface{})
	if event.Opened && !event.Closed {
		value["status"] = "loading"
	} else if event.Closed {
		value["status"] = "ready"
	}

	if _, ok := value["status"]; ok {
		data, _ := json.Marshal(value)
		values["value"] = base64.StdEncoding.EncodeToString([]byte(data))
	}
	jsonValue, _ := json.Marshal(values)
	return jsonValue
}

// Filler : Struct for managing multiple event paths
type Filler struct {

	// A map of paths and events to watch
	Paths map[string]*FilledFileEvent

	// Configuration item for the fill command
	Config *config.Config
}

// NewFiller : Create a new filler instance for adding/deleting from the database
func NewFiller(config *config.Config) *Filler {
	filler := Filler{}
	filler.Config = config
	filler.Paths = make(map[string]*FilledFileEvent)
	for i := 1; i <= MAXCLIENTS; i++ {
		go filler.requestMaker()
	}
	return &filler
}

// Add : Add an event to the database
func (filler *Filler) Add(bucket string, dirname string, filename string, notification notify.Event) {
	var path string = filepath.Join(dirname, filename)
	if dirname == bucket {
		filename = "root:" + filename
	} else {
		filename = dirname + ":" + filename
	}
	bucket = "files/" + bucket

	if _, ok := filler.Paths[path]; !ok {
		filler.Paths[path] = NewFillEvent(filler.Config, bucket, filename)
	}

	if event := NewFillEvent(filler.Config, bucket, filename).State(notification); event != nil {
		if event.Deleted {
			go event.Delete()
			delete(filler.Paths, path)
			log.Info("Deleted ", bucket, "/", filename)
		} else {
			go event.Store()
			log.Info("Stored ", bucket, "/", filename)
			return
		}
	}
}

// Read requests off the channel and send them to the assemble server
//
// Starts MAX_CLIENTS http clients and tries to send each request up to 5 times
// to ensure delivery of the payload.
func (filler *Filler) requestMaker() {
	var client = &http.Client{
		Timeout: config.TIMEOUT,
	}
	var (
		maxRetries int = 5
		retries    int = maxRetries
		err        error
		response   *http.Response
	)
	for {
		select {
		case request := <-requests:
			for retries > 0 {
				response, err = client.Do(request)
				if err == nil {
					break
				}
				retries--
			}
			if response != nil {
				if err := response.Body.Close(); err != nil {
					log.Println(err)
				}
			}
		}
	}
}
