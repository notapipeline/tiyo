// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/notapipeline/tiyo/pkg/pipeline"
	log "github.com/sirupsen/logrus"
)

// QueueItem : Type information about an item on the queue
type QueueItem struct {

	// The pipeline folder this queue is destined for
	PipelineFolder string `json:"pipelineFolder"`

	// Subfolder is a child of pipeline folder and usually
	// representative of the upstream command being actioned
	SubFolder string `json:"subFolder"`

	// The filename of the queue element
	Filename string `json:"filename"`

	// Event data represented in string format
	Event string `json:"event"`

	// The command to execute
	Command pipeline.Command `json:"command"`
}

// PopQueue : Take an item off the queue
//
// INTERNAL used for comms between flow and assemble
//
// GET /queue/:pipeline/:key
func (api *API) PopQueue(c *gin.Context) {
	result := Result{
		Code:   200,
		Result: "OK",
	}

	var (
		pipelineName string = c.Params.ByName("pipeline")
		key          string = c.Params.ByName("key")
	)
	pipeline, err := pipeline.GetPipeline(api.Config, pipelineName)
	if err != nil {
		result.Code = 500
		result.Result = "Error"
		result.Message = "Error opening pipeline " + pipelineName + " " + err.Error()
		c.JSON(result.Code, result)
		return
	}
	log.Info("PopQueue Using pipeline ", pipeline)

	var (
		// key = 'container:version:hostname'
		// example: 'python:3.19-alpine-3.12:example-pipeline-test-0'
		keyparts  []string = strings.Split(key, ":")
		container string   = keyparts[0]
		version   string   = keyparts[1]

		// takes a cluster object name (example: example-pipeline-test) and strips the pipeline name
		// leaving just 'test' which should be the container name + ID (test-0 test-adf8bc4)
		hostname    string = strings.Trim(strings.TrimPrefix(keyparts[2], pipeline.DNSName), "-")
		activeKey   string
		activeIndex int
		group       string
	)

	queue := make(map[string]string)
	// the last element is the pod index/ID

	// now get language:version:container
	m := map[string]string{}
	matches := containerNameMatcher.FindAllStringSubmatch(hostname, -1)
	for i, n := range matches[0] {
		m[names[i]] = n
	}
	if m["deployment"] != "" {
		group = m["deployment"]
	} else if m["container"] != "" {
		group = m["container"]
	}

	if group == "" {
		log.Error("Invalid group name for key ", key)
	}

	// rebuild the key
	key = fmt.Sprintf("%s:%s:%s", container, version, group)
	if err := api.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("queue")).Bucket([]byte(pipeline.BucketName))
		log.Debug("PopQueue Scanning for key ", key)
		c := b.Cursor()
		for k, v := c.Seek([]byte(key)); k != nil && bytes.HasPrefix(k, []byte(key)); k, v = c.Next() {
			queue[string(k)] = string(v)
		}
		// if len queue is 0, we have too much data in key
		// workaround, pop after the last - and rescan
		// however this is inefficient and we need to understand
		// a better way of getting informaton out of flow about
		// the container we're retrieving for.
		if len(queue) == 0 {
			if index := strings.LastIndex(group, "-"); index != -1 {
				group = group[:index]
			}
			c = b.Cursor()
			for k, v := c.Seek([]byte(key)); k != nil && bytes.HasPrefix(k, []byte(key)); k, v = c.Next() {
				queue[string(k)] = string(v)
			}
		}

		return nil
	}); err != nil {
		log.Error(err)
	}
	log.Debug(queue)

	// take the first element off the queue and then delete it from the database

	// To prevent a race condition across api calls, we use a mutex lock on a keyslice
	// this means we can safely handle handing commands out to the pods without
	// multiple pods receiving the same event
	api.queueLock.Lock()
	for activeKey = range queue {
		var found bool = false
		for _, check := range api.queueLock.locks {
			if check == activeKey {
				found = true
				break
			}
		}
		if !found && activeKey != "" {
			api.queueLock.locks = append(api.queueLock.locks, activeKey)
			break
		}
	}
	api.queueLock.Unlock()
	if activeKey == "" {
		result.Code = 202
		result.Message = ""
		c.JSON(result.Code, result)
		return
	}

	if err := api.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("queue")).Bucket([]byte(pipeline.BucketName))
		if err := b.Delete([]byte(activeKey)); err != nil {
			return fmt.Errorf("Error deleting key %s - %s", activeKey, err)
		}
		return nil
	}); err != nil {
		log.Error(err)
	}

	// update files bucket to store state
	slice := strings.Split(activeKey, ":")
	keystr := slice[len(slice)-2] + ":" + slice[len(slice)-1]
	log.Warn(keystr)
	if err := api.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("files")).Bucket([]byte(pipeline.BucketName))
		var tag string = container + ":" + version
		log.Debug("Updating state in files/", pipeline.BucketName, " for container ", tag)
		value := b.Get([]byte(keystr))
		body, _ := base64.StdEncoding.DecodeString(string(value))
		content := make(map[string]string)
		json.Unmarshal(body, &content)
		content[tag] = "in_progress"
		body, _ = json.Marshal(content)
		err = b.Put([]byte(keystr), []byte(base64.StdEncoding.EncodeToString([]byte(body))))
		if err != nil {
			return fmt.Errorf("create kv: %s", err)
		}

		return nil
	}); err != nil {
		log.Error(err)
	}

	// now remove it from the lock
	if len(api.queueLock.locks) > 0 {
		locks := api.queueLock.locks
		var element string
		for activeIndex, element = range locks {
			if element == activeKey {
				break
			}
		}
		api.queueLock.Lock()
		locks[len(locks)-1], locks[activeIndex] = locks[activeIndex], locks[len(locks)-1]
		api.queueLock.locks = locks[:len(locks)-1]
		api.queueLock.Unlock()
	}

	var id string = queue[activeKey]
	var str []string = strings.Split(activeKey, ":")
	if len(str) == 0 {
		result.Code = 202
		result.Message = ""
		c.JSON(result.Code, result)
		return
	}

	// Now build the response
	message := QueueItem{
		PipelineFolder: pipeline.BucketName,
		// get subfolder or "" if subfolder is root
		SubFolder: strings.TrimPrefix(str[len(str)-2], "root"),
		Filename:  str[len(str)-1],
		Command:   *pipeline.Commands[id],
	}
	result.Message = message
	c.JSON(result.Code, result.Message)
}

// PerpetualQueue : Updates the given queue with new event items
//
// INTERNAL - Automatically update the queue with new items
//
// GET /perpetualqueue
//
// Request parameters:
// - pipeline The pipeline to execute
// - maxitems The max items to update into the queue
//
// Response codes
// - 202 Accept
// - 500 Internal server error
func (api *API) PerpetualQueue(c *gin.Context) {
	request := make(map[string]interface{})
	if err := c.ShouldBind(&request); err != nil {
		request["pipeline"] = c.PostForm("pipeline")
		request["maxitems"] = c.PostForm("maxitems")
	}
	log.Info("Queue request ", request)
	result := Result{
		Code:    202,
		Result:  "OK",
		Message: "OK",
	}

	var (
		pipelineName string
		maxitems     int
	)

	if _, ok := request["pipeline"]; ok {
		pipelineName = request["pipeline"].(string)
	}
	if _, ok := request["maxitems"]; ok {
		maxitems = int(request["maxitems"].(float64))
	}

	pipeline, err := pipeline.GetPipeline(api.Config, pipelineName)
	if err != nil {
		result.Code = 500
		result.Result = "Error"
		result.Message = "Error opening pipeline " + pipelineName + " " + err.Error()
		c.JSON(result.Code, result)
		return
	}
	log.Debug("Using pipeline ", pipeline)

	if _, ok := api.QueueSize[pipeline.BucketName]; !ok {
		api.QueueSize[pipeline.BucketName] = maxitems
	}

	var count int = 0
	if err := api.Db.View(func(tx *bolt.Tx) error {
		log.Infof("Opening queue bucket `%s`", pipeline.BucketName)
		b := tx.Bucket([]byte("queue")).Bucket([]byte(pipeline.BucketName))
		if b == nil {
			return fmt.Errorf("No such bucket `queue` for pipeline %s", pipelineName)
		}
		log.Debug("Starting queue count for ", pipelineName)
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			count++
		}
		return nil
	}); err != nil {
		log.Error(err)
	}

	log.Debug("Got ", count, " of ", api.QueueSize[pipeline.BucketName])
	// get all files available for this command
	if count < api.QueueSize[pipeline.BucketName] {
		log.Debug("Starting walk for ", pipelineName)
		available := api.QueueSize[pipeline.BucketName] - count
		for _, command := range pipeline.GetStart() {
			// walk pipeline from here
			api.walkFiles(pipeline, command, &available)
		}
	}

	c.JSON(result.Code, result)
}

// Walks files expected from a pipeline and updates these into the queue bucket
func (api *API) walkFiles(pipeline *pipeline.Pipeline, command *pipeline.Command, count *int) {
	log.Debug("Walking ", command.Name, " ", command.ID)
	sources := pipeline.GetPathSources(command)
	available := make(map[string]map[string]string)
	for _, source := range sources {
		var bucket []byte = []byte("files")
		if err := api.Db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket(bucket).Bucket([]byte(pipeline.BucketName))
			if b == nil {
				return fmt.Errorf("No such bucket for queue `%s`", pipeline.BucketName)
			}
			c := b.Cursor()
			for k, v := c.Seek([]byte(source)); k != nil && bytes.HasPrefix(k, []byte(source)); k, v = c.Next() {
				if v != nil {
					log.Debug("Appending ", k, "to available files")
					body, _ := base64.StdEncoding.DecodeString(string(v))
					content := make(map[string]string)
					_ = json.Unmarshal(body, &content)
					available[string(k)] = content
					if len(available) >= *count {
						break
					}
				}
			}
			return nil
		}); err != nil {
			log.Error(err)
		}

		tag := command.GetContainer(true)
		// clear any statuses that are not "ready"
		for k, content := range available {
			// dont use file load status but check for our own...
			// valid == missing|"ready"
			if _, ok := content[tag]; ok {
				if content[tag] != "ready" {
					delete(available, k)
				}
			}
		}

		// Add to queue
		added := make([]string, 0)
		if err := api.Db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("queue")).Bucket([]byte(pipeline.BucketName))

			for k := range available {
				// need command container name as second
				key := tag + ":" + pipeline.GetParent(command).Name + ":" + k
				err := b.Put([]byte(key), []byte(command.ID))
				if err != nil {
					return fmt.Errorf("create kv: %s", err)
				}
				added = append(added, k)
				*count--
				if *count == 0 {
					break
				}
			}
			return nil
		}); err != nil {
			log.Error(err)
		}

		// update files bucket to store state
		if err := api.Db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("files")).Bucket([]byte(pipeline.BucketName))
			for _, v := range added {
				log.Debug("Adding ", command.ID, "to files/", pipeline.BucketName)
				value := available[v]
				value[tag] = "queued"
				com, _ := json.Marshal(value)
				body := base64.StdEncoding.EncodeToString([]byte(com))
				err := b.Put([]byte(v), []byte(body))
				if err != nil {
					return fmt.Errorf("create kv: %s", err)
				}
			}

			return nil
		}); err != nil {
			log.Error(err)
		}
	}
	if *count != 0 {
		for _, com := range pipeline.GetNext(command) {
			if com != nil { // disconnected link
				api.walkFiles(pipeline, com, count)
			} else {
				log.Debug("Not walking path from ", command.Name, " - Link is disconnected")
			}
		}
	}
}
