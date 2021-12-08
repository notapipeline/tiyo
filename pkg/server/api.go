// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

// Primary API service for Assemble server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/notapipeline/tiyo/pkg/config"
	"github.com/notapipeline/tiyo/pkg/pipeline"
	log "github.com/sirupsen/logrus"
)

// Store the list of containers globally to prevent it
// being re-downloaded each time the list is requested
var containers []string

// GithubResponse : Expected information from the Github request
type GithubResponse struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

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

// Result : A server result defines the body wrapper sent back to the client
type Result struct {

	// HTTP response code
	Code int `json:"code"`

	// HTTP Result string - normally one of OK | Error
	Result string `json:"result"`

	// The request message being returned
	Message interface{} `json:"message"`
}

// ScanResult : Return values stored in a bucket
type ScanResult struct {

	// Buckets is a list of child buckets the scanned bucket might contain
	Buckets []string `json:"buckets"`

	// For simplicity on the caller side, send the number of child buckets found
	BucketsLength int `json:"bucketlen"`

	// A map of key:value pairs containing bucket data
	Keys map[string]string `json:"keys"`

	// For simplicity, the number of keys in the bucket
	KeyLen int `json:"keylen"`
}

// Lock : Allow for locking individual queues whilst manipulating them
type Lock struct {
	sync.Mutex
	locks []string
}

// NewResult : Create a new result item
func NewResult() *Result {
	result := Result{}
	return &result
}

// API : Assemble server api object
type API struct {

	// The bolt database held by this installation
	Db *bolt.DB

	// Server Configuration
	Config *config.Config

	// A map of queue sizes
	QueueSize map[string]int

	// The lock table for the queues
	queueLock *Lock
}

// NewAPI : Create a new API instance
func NewAPI(dbName string, config *config.Config) (*API, error) {
	api := API{}
	api.Config = config
	var err error
	api.Db, err = bolt.Open(dbName, 0600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}
	api.QueueSize = make(map[string]int)
	lock := Lock{
		locks: make([]string, 0),
	}
	api.queueLock = &lock
	return &api, nil
}

// Index : Render the index page back on the GIN context
//
// TODO : Make this a little more versatile and use SSR to render
//        more of the page than relying on JS and a one page website
func (api *API) Index(c *gin.Context) {
	c.HTML(200, "index", gin.H{
		"Title": "TIYO ASSEMBLE - Kubernetes cluster designer",
	})
}

// GetContainers : Get the list of containers for the sidebar on the pipeline page
func (api *API) GetContainers() []string {
	if containers != nil {
		return containers
	}
	request, err := http.NewRequest("GET", "https://api.github.com/repos/BioContainers/containers/contents", nil)
	if err != nil {
		panic(err)
	}
	request.Header.Set("Accept", "application/vnd.github.v3+json")
	client := &http.Client{
		Timeout: config.TIMEOUT,
	}
	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	body, _ := ioutil.ReadAll(response.Body)

	message := make([]GithubResponse, 0)
	json.Unmarshal(body, &message)
	for index := range message {
		if message[index].Type == "dir" {
			containers = append(containers, message[index].Name)
		}
	}
	return containers
}

// Containers : The API endpoint method for retrieving the sidebar container set
//
// Returned responses will be one of:
// - 200 OK    : Message will be a list of strings
func (api *API) Containers(c *gin.Context) {
	result := NewResult()
	result.Code = 200
	result.Result = "OK"
	result.Message = api.GetContainers()
	c.JSON(result.Code, result)
}

// Buckets : List all available top level buckets
//
// Returned responses will be one of:
// - 200 OK    : Message will be a list of bucket names
// - 500 Error : Message will be the error message
func (api *API) Buckets(c *gin.Context) {
	result := NewResult()
	result.Code = 200
	result.Result = "OK"
	result.Message = make([]string, 0)
	if err := api.Db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			b := []string{string(name)}
			result.Message = append(
				result.Message.([]string), b...)
			return nil
		})
	}); err != nil {
		result = NewResult()
		result.Code = 500
		result.Result = "Error"
		result.Message = fmt.Sprintf("%s", err)
	}
	c.JSON(result.Code, result)
}

// CreateBucket : Create a new bucket
//
// POST /bucket
//
// The create request must contain:
// - bucket - the name of the bucket to create or parent bucket name when creating a child
// - child  - [optional] the name of the child bucket to create
//
// Responses are:
// - 201 Created
// - 400 Bad request sent when bucket name is empty
// - 500 Internal server error
func (api *API) CreateBucket(c *gin.Context) {
	result := NewResult()
	result.Code = 201
	result.Result = "OK"

	request := make(map[string]string)
	if err := c.ShouldBind(&request); err != nil {
		request["bucket"] = c.PostForm("bucket")
		request["child"] = c.PostForm("child")
		if request["child"] == "" {
			delete(request, "child")
		}
	}

	if request["bucket"] == "" {
		result.Code = 400
		result.Result = "Error"
		result.Message = "No bucket name provided"
		c.JSON(result.Code, result)
		return
	}

	if err := api.Db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(request["bucket"]))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		if child, ok := request["child"]; ok {
			if _, err = bucket.CreateBucketIfNotExists([]byte(child)); err != nil {
				return fmt.Errorf("Error creating child bucket %s", child)
			}
		}
		return nil
	}); err != nil {
		result.Code = 500
		result.Result = "Error"
		result.Message = fmt.Sprintf("%s", err)
	}
	c.JSON(result.Code, result)
}

// DeleteBucket : Deletes a given bucket
//
// DELETE /bucket/:bucket[/:child]
//
// The url parameters must contain:
// - bucket - the name of the bucket to delete or parent bucket name when deleting a child
// - child  - [optional] the name of the child bucket to create
//
// Responses are:
// - 202 Accepted
// - 400 Bad request sent when bucket name is empty
// - 500 Internal server error
func (api *API) DeleteBucket(c *gin.Context) {
	result := NewResult()
	result.Code = 202
	result.Result = "OK"

	var bucket = c.Params.ByName("bucket")
	if bucket == "" {
		result.Code = 400
		result.Result = "Error"
		result.Message = "No bucket name provided"
		c.JSON(result.Code, result)
		return
	}

	if err := api.Db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket([]byte(bucket))
		return err
	}); err != nil {
		result.Code = 500
		result.Result = "Error"
		result.Message = fmt.Sprintf("%s", err)
	}
	if result.Code == 202 {
		c.JSON(result.Code, nil)
	} else {
		c.JSON(result.Code, result)
	}

}

// DeleteKey : Delete a key value pair from a bucket
//
// DELETE /bucket/:bucket[/:child]/:key
//
// URL parameters are:
// - bucket : The name of the bucket containing the key
// - child  : [optional] The name of a child bucket to manage
// - key    : The key to delete
//
// Returned responses are:
// - 202 Accepted
// - 400 Bad Request when bucket or key are empty
// - 500 Internal server error on failure to delete
func (api *API) DeleteKey(c *gin.Context) {
	result := NewResult()
	result.Code = 202
	result.Result = "OK"

	request := make(map[string]string)
	request["bucket"] = c.Params.ByName("bucket")
	request["child"] = c.Params.ByName("child")
	request["key"] = c.Params.ByName("key")

	if request["key"] == "" {
		request["key"] = request["child"]
		delete(request, "child")
	}
	request["key"] = strings.Trim(request["key"], "/")

	if request["bucket"] == "" || request["key"] == "" {
		result.Code = 400
		result.Result = "Error"
		result.Message = "Missing bucket name or key"
		c.JSON(result.Code, result)
		return
	}

	if err := api.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(request["bucket"]))
		if b == nil {
			return fmt.Errorf("No such bucket")
		}
		if child, ok := request["child"]; ok {
			b = b.Bucket([]byte(child))
		}

		if val := b.Get([]byte(request["key"])); val == nil {
			if err := b.DeleteBucket([]byte(request["key"])); err != nil {
				return fmt.Errorf("Error deleting inner bucket %s - %s", request["key"], err)
			}
		}

		if err := b.Delete([]byte(request["key"])); err != nil {
			return fmt.Errorf("Error deleting key %s - %s", request["key"], err)
		}
		return nil
	}); err != nil {
		log.Println(err)
		result.Code = 500
		result.Result = "Error"
		result.Message = err
	}
	if result.Code == 202 {
		c.JSON(result.Code, nil)
	} else {
		c.JSON(result.Code, result)
	}

}

// Put : create a key/value pair in the boltdb
//
// PUT /bucket
//
// Request parameters:
// - bucket The bucket to write into
// - child  [optional] the child bucket to write into
// - key    The key to add
// - value  The value to save against the key
//
// Response codes
// - 204 No content if key successfully stored
// - 400 Bad request if bucket or key is missing
// - 500 Internal server error if value cannot be stored
func (api *API) Put(c *gin.Context) {
	result := NewResult()
	result.Code = 204
	result.Result = "OK"

	request := make(map[string]string)
	if err := c.ShouldBind(&request); err != nil {
		request["bucket"] = c.PostForm("bucket")
		request["key"] = c.PostForm("key")
		if c.PostForm("child") != "" {
			request["child"] = c.PostForm("child")
		}
		request["value"] = c.PostForm("value")
	}

	if child, ok := request["child"]; ok {
		if child == "" {
			delete(request, "child")
		}
	}

	if request["bucket"] == "" || request["key"] == "" {
		result.Code = 400
		result.Result = "Error"
		result.Message = "Missing bucket name or key"
		c.JSON(result.Code, result)
		return
	}

	if err := api.Db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(request["bucket"]))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}

		if child, ok := request["child"]; ok {
			b = b.Bucket([]byte(child))
		}

		//log.Debug(request, b)
		err = b.Put([]byte(request["key"]), []byte(request["value"]))
		if err != nil {
			return fmt.Errorf("create kv: %s", err)
		}

		return nil
	}); err != nil {
		result.Code = 500
		result.Result = "Error"
		result.Message = err
	}
	if result.Code == 204 {
		c.JSON(result.Code, nil)
	} else {
		c.JSON(result.Code, result)
	}

}

// Get : Get the value for a given key
//
// GET /bucket/:bucket[/:child]/:key
//
// Response codes
// - 200 OK - Message will be the value
// - 400 Bad Request if bucket or key is empty
// - 500 Internal server error if value cannot be retrieved
func (api *API) Get(c *gin.Context) {
	result := NewResult()
	result.Code = 200
	result.Result = "OK"

	request := make(map[string]string)
	request["bucket"] = c.Params.ByName("bucket")
	request["child"] = c.Params.ByName("child")
	request["key"] = c.Params.ByName("key")
	if request["key"] == "" {
		request["key"] = request["child"]
		delete(request, "child")
	}
	request["key"] = strings.Trim(request["key"], "/")

	if request["bucket"] == "" || request["key"] == "" {
		result.Code = 400
		result.Result = "Error"
		result.Message = "Missing bucket name or key"
		c.JSON(result.Code, result)
		return
	}

	if err := api.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(request["bucket"]))
		if b == nil {
			return fmt.Errorf("No such bucket")
		}

		if child, ok := request["child"]; ok {
			b = b.Bucket([]byte(child))
		}

		value := b.Get([]byte(request["key"]))
		if value == nil {
			return fmt.Errorf("Key not found")
		}
		result.Message = string(value)
		return nil
	}); err != nil {
		result.Code = 500
		result.Result = "Error"
		result.Message = err
	}
	c.JSON(result.Code, result)

}

// PrefixScan : Scan a bucket and retrieve all contents with keys matching prefix
//
// GET /scan/:bucket[/:child][/:key]
//
// Request parameters
// - bucket [required] The bucket name to scan
// - child  [optional] The child bucket to scan instead
// - key    [optional] If key is not specified, all contents will be returned
//
// Response codes
// - 200 OK Messsage will be a map of matching key/value pairs
// - 400 Bad Request if bucket name is empty
// - 500 Internal server error if the request fails for any reason
func (api *API) PrefixScan(c *gin.Context) {
	result := Result{Result: "error"}
	result.Code = 200
	result.Result = "OK"
	result.Message = make(map[string]interface{})
	request := make(map[string]string)

	request["bucket"] = strings.Trim(c.Params.ByName("bucket"), "/")
	if c.Params.ByName("child") != "" {
		request["child"] = strings.Trim(c.Params.ByName("child"), "/")
	}

	if c.Params.ByName("key") != "" {
		request["key"] = strings.Trim(c.Params.ByName("key"), "/")
	}

	if request["bucket"] == "" {
		result.Code = 400
		result.Result = "Error"
		result.Message = "no bucket name"
		c.JSON(result.Code, result)
		return
	}

	scanResults := ScanResult{}
	scanResults.Buckets = make([]string, 0)
	scanResults.BucketsLength = 0
	scanResults.Keys = make(map[string]string)
	scanResults.KeyLen = 0

	if err := api.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(request["bucket"]))

		var key string = ""
		if child, ok := request["child"]; ok {
			b = b.Bucket([]byte(child))
			if _, ok = request["key"]; ok {
				key = request["key"]
			}
		}
		if b == nil {
			return fmt.Errorf("No such bucket or bucket is invalid")
		}
		c := b.Cursor()

		if key != "" {
			prefix := []byte(key)
			for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
				if v == nil {
					scanResults.Buckets = append(scanResults.Buckets, string(k))
					scanResults.BucketsLength++
				} else {
					scanResults.Keys[string(k)] = string(v)
					scanResults.KeyLen++
				}
			}
		} else {
			for k, v := c.First(); k != nil; k, v = c.Next() {
				if v == nil {
					scanResults.Buckets = append(scanResults.Buckets, string(k))
					scanResults.BucketsLength++
				} else {
					scanResults.Keys[string(k)] = string(v)
					scanResults.KeyLen++
				}
			}
		}
		return nil
	}); err != nil {
		log.Error(err)
		result.Code = 500
		result.Result = "Error"
		result.Message = err.Error()
	}
	result.Message = scanResults
	c.JSON(result.Code, result)

}

// KeyCount : Count keys in a bucket
//
// GET /count/:bucket[/:child]
//
// Response codes:
// - 200 OK - Message will be the number of keys in the bucket
// - 400 Bad Request - Bucket name is missing
// - 404 Page not found - Bucket name is invalid
// - 500 Internal server error on all other errors
func (api *API) KeyCount(c *gin.Context) {
	result := Result{Result: "error"}
	result.Code = 200
	result.Result = "OK"

	request := make(map[string]string)
	request["bucket"] = strings.Trim(c.Params.ByName("bucket"), "/")
	if c.Params.ByName("child") != "" {
		request["child"] = strings.Trim(c.Params.ByName("child"), "/")
	}

	if request["bucket"] == "" {
		result.Code = 400
		result.Result = "Error"
		result.Message = "no bucket name"
		c.JSON(result.Code, result)
		return
	}

	var count int = 0
	if err := api.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(request["bucket"]))
		if b != nil {
			if child, ok := request["child"]; ok {
				b = b.Bucket([]byte(child))
			}
			c := b.Cursor()
			for k, _ := c.First(); k != nil; k, _ = c.Next() {
				count++
			}
			result.Message = count
		} else {
			result.Code = 404
			result.Result = "Error"
			result.Message = "no such bucket available"
		}
		return nil
	}); err != nil {
		result.Code = 500
		result.Result = "Error"
		result.Message = err
	}
	c.JSON(result.Code, result)
}

// ExecuteFlow : Executes the current pipeline
//
// POST /execute
//
// Request parameters:
// - pipeline - The name of the pipeline to trigger
//
// Response codes:
// - See forwardPost method below
//
// Flow execution is handed off to the flow api to build the infrastructure
// and begin executing the queue.
//
// This should be a straight pass-through and flow should be responsible for
// verifying if infrastructure has/has not already been built or the pipeline
// is already in the process of being executed.
func (api *API) ExecuteFlow(c *gin.Context) {
	result, _, _ := api.forwardPost(c, "execute")
	c.JSON(result.Code, result)
}

// StopFlow : Stops the queue to prevent flow of information
//
// POST /stopflow
//
// Request parameters:
// - pipeline - The name of the pipeline to trigger
//
// Response codes:
// - See forwardPost method below
func (api *API) StopFlow(c *gin.Context) {
	result, _, _ := api.forwardPost(c, "stop")
	c.JSON(result.Code, result)
}

// StartFlow : Starts the queue to flow information out into the pods
//
// POST /startflow
//
// Request parameters:
// - pipeline - The name of the pipeline to trigger
//
// Response codes:
// - See forwardPost method below
func (api *API) StartFlow(c *gin.Context) {
	result, _, _ := api.forwardPost(c, "start")
	c.JSON(result.Code, result)
}

// DestroyFlow : Destroys all infrastructure for a given flow
//
// POST /destroyflow
//
// Request parameters:
// - pipeline - The name of the pipeline to trigger
//
// Response codes:
// - See forwardPost method below
func (api *API) DestroyFlow(c *gin.Context) {
	result, content, err := api.forwardPost(c, "destroy")
	if err != nil {
		c.JSON(result.Code, result)
		log.Warn("Not cleaning up")
		return
	}

	var pipelineName string = pipeline.Sanitize(content["pipeline"], "_")
	for _, name := range []string{"events", "files", "pods", "queue"} {
		if err := api.Db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(name))
			if b == nil {
				return fmt.Errorf("No such bucket")
			}

			if val := b.Get([]byte(pipelineName)); val == nil {
				if err := b.DeleteBucket([]byte(pipelineName)); err != nil {
					return fmt.Errorf("Error deleting inner bucket %s - %s", pipelineName, err)
				}
			}

			if err := b.Delete([]byte(pipelineName)); err != nil {
				return fmt.Errorf("Error deleting key %s - %s", pipelineName, err)
			}

			if _, err = b.CreateBucketIfNotExists([]byte(pipelineName)); err != nil {
				return fmt.Errorf("Error creating inner bucket %s/%s", name, pipelineName)
			}
			return nil
		}); err != nil {
			result.Code = 500
			result.Result = "Error"
			result.Message = err
		}
	}
	c.JSON(result.Code, result)
}

// forwardPost : Manages forwarding requests from the client through to Flow
//
// Response codes:
// - 400 Bad request if request cannot bind to map[string]string
// - 500 Internal server error if request or response are invalid
// - All others are reponse codes from the related endpoints in flow
func (api *API) forwardPost(c *gin.Context, endpoint string) (Result, map[string]string, error) {
	content := make(map[string]string)
	if err := c.ShouldBind(&content); err != nil {
		log.Error("Failed to bind content ", err)
		result := Result{
			Code:    400,
			Result:  "Error",
			Message: err.Error(),
		}
		return result, content, err
	}
	log.Debug(content)
	data, _ := json.Marshal(content)

	serverAddress := api.Config.FlowServer()
	request, err := http.NewRequest(
		http.MethodPost,
		serverAddress+"/api/v1/"+endpoint,
		bytes.NewBuffer(data))
	if err != nil {
		log.Error("Failed to build the request ", err)
		result := Result{
			Code:    500,
			Result:  "Error",
			Message: err.Error(),
		}
		return result, content, err
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Connection", "close")
	request.Close = true
	client := &http.Client{
		Timeout: config.TIMEOUT,
	}
	response, err := client.Do(request)
	if err != nil {
		log.Error("Client request failed ", err)
		result := Result{
			Code:    500,
			Result:  "Error",
			Message: err.Error(),
		}
		return result, content, err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Error("Failed to read body ", err)
		result := Result{
			Code:    500,
			Result:  "Error",
			Message: err.Error(),
		}
		return result, content, err
	}

	ncontent := Result{}
	err = json.Unmarshal(body, &ncontent)
	if err != nil {
		log.Error("Failed to unmarshal JSON object ", err)
		result := Result{
			Code:    500,
			Result:  "Error",
			Message: err.Error(),
		}
		return result, content, err
	}
	return ncontent, content, nil
}

// FlowStatus : Get the status of all items in the current pipeline
//
// Request params
// - pipeline : The pipeline to get status messages for
//
// Response codes:
// - 200 OK - Statuses will be the message field in the response
// - 500 if status cannot be retrieved
func (api *API) FlowStatus(c *gin.Context) {
	res := make(map[string]string)
	res["pipeline"] = c.Params.ByName("pipeline")
	data, _ := json.Marshal(res)

	serverAddress := api.Config.FlowServer()
	request, err := http.NewRequest(
		http.MethodPost,
		serverAddress+"/api/v1/status",
		bytes.NewBuffer(data))
	if err != nil {
		log.Error(err)
		result := Result{
			Code:    500,
			Result:  "Error",
			Message: err.Error(),
		}
		c.JSON(result.Code, result)
		return
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Connection", "close")
	request.Close = true
	client := &http.Client{
		Timeout: config.TIMEOUT,
	}
	response, err := client.Do(request)
	if err != nil {
		log.Error(err)
		result := Result{
			Code:    500,
			Result:  "Error",
			Message: err.Error(),
		}
		c.JSON(result.Code, result)
		return
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	content := Result{}
	err = json.Unmarshal(body, &content)
	if err != nil {
		log.Error(err)
		result := Result{
			Code:    500,
			Result:  "Error",
			Message: err.Error(),
		}
		c.JSON(result.Code, result)
		return
	}
	c.JSON(content.Code, content)
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
	log.Debug("Using pipeline ", pipeline)

	var (
		// key = 'container:version:hostname'
		// example: 'python:3.19-alpine-3.12:example-pipeline-test-0'
		keyparts  []string = strings.Split(key, ":")
		container string   = keyparts[0]
		version   string   = keyparts[1]

		// takes a cluster object name (example: example-pipeline-test) and strips the pipeline name
		// leaving just 'test' which should be the container name + ID (test-0, test-adf8bc4)
		group       string = strings.Trim(strings.TrimPrefix(keyparts[2], pipeline.DNSName), "-")
		activeKey   string
		activeIndex int
	)

	queue := make(map[string]string)
	// the last element is the pod index/ID
	if index := strings.LastIndex(group, "-"); index != -1 {
		group = group[:index]
	}
	// rebuild the key
	key = fmt.Sprintf("%s:%s:%s", container, version, group)
	if err := api.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("queue")).Bucket([]byte(pipeline.BucketName))
		log.Debug("Scanning for key ", key)
		c := b.Cursor()
		for k, v := c.Seek([]byte(key)); k != nil && bytes.HasPrefix(k, []byte(key)); k, v = c.Next() {
			queue[string(k)] = string(v)
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
	log.Debug(str, len(str))
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

// Encrypt : Encrypt a message and send back the encrypted value as a base64 encoded string
//
// POST /encrypt
//
// Request parameters
// - value : the value to encrypt
//
// Response codes
// 200 OK message will contain encrypted value
// 400 Bad request if value is empty
// 500 Internal server error if message cannot be encrypted
func (api *API) Encrypt(c *gin.Context) {
	request := make(map[string]string)
	var err error
	if err = c.ShouldBind(&request); err != nil {
		request["value"] = c.PostForm("value")
	}

	if val, ok := request["value"]; !ok || val == "" {
		result := Result{
			Code:    400,
			Result:  "Error",
			Message: fmt.Sprintf("Bad request whilst encrypting passphrase"),
		}
		c.JSON(result.Code, result)
		return
	}

	var passphrase []byte
	if passphrase, err = encrypt([]byte(request["value"]), api.Config.GetPassphrase("assemble")); err != nil {
		result := Result{
			Code:    500,
			Result:  "Error",
			Message: err.Error(),
		}
		c.JSON(result.Code, result)
		return
	}

	result := Result{
		Code:    200,
		Result:  "OK",
		Message: base64.StdEncoding.EncodeToString(passphrase),
	}
	c.JSON(result.Code, result)
}

// Decrypt Decrypts a given string and sends back the plaintext value
//
// INTERNAL Execution only
//
// POST /decrypt
//
// Request parameters:
// - value - the value to decrypt
// - token - the validation token to ensure decryption is allowed to take place
//
// Response codes:
// - 200 OK Message will be the decrypted value
// - 400 Bad request if value is empty, token is invalid or token matches value
// - 500 internal server error if value cannot be decoded - Message field may offer further clarification
//
// This function requires both a value and a 'token' to be passed in
// via context.
//
// token is the encrypted version of the passphrase used to encrypt passwords
// and should only be available to the flow server.
//
// This is to offer an additional level of security at the browser level to prevent
// attackers from decrypting a user stored password by accessing the decrypt api
// endpoint to have the server decrypt the password for them.
func (api *API) Decrypt(c *gin.Context) {
	request := make(map[string]string)
	var (
		err            error
		value          string
		token          string
		ok             bool
		decoded        []byte
		assemblePhrase string = api.Config.GetPassphrase("assemble")
	)

	if err = c.ShouldBind(&request); err != nil {
		request["value"] = c.PostForm("value")
		request["token"] = c.PostForm("token")
	}

	if value, ok = request["value"]; !ok || value == "" {
		result := Result{
			Code:    400,
			Result:  "Error",
			Message: "Bad request whilst decrypting data - missing value",
		}
		c.JSON(result.Code, result)
		return
	}

	if token, ok = request["token"]; !ok || token == "" || token == value {
		result := Result{
			Code:    400,
			Result:  "Error",
			Message: "Missing token for decryption or token matches password",
		}
		c.JSON(result.Code, result)
		return
	}

	decoded, _ = base64.StdEncoding.DecodeString(token)
	if decoded, err = decrypt(decoded, assemblePhrase); err != nil || string(decoded) != assemblePhrase {
		result := Result{
			Code:    400,
			Result:  "Error",
			Message: "Failed to decode token or token is invalid",
		}
		c.JSON(result.Code, result)
		return
	}
	var passphrase []byte
	passphrase, _ = base64.StdEncoding.DecodeString(value)
	if passphrase, err = decrypt(passphrase, assemblePhrase); err != nil {
		result := Result{
			Code:    500,
			Result:  "Error",
			Message: err.Error(),
		}
		c.JSON(result.Code, result)
		return
	}

	result := Result{
		Code:    200,
		Result:  "OK",
		Message: string(passphrase),
	}
	c.JSON(result.Code, result)
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
		b := tx.Bucket([]byte("queue")).Bucket([]byte(pipeline.BucketName))
		log.Debug("Starting queue count for ", pipelineName)
		c := b.Cursor()
		log.Debug(c)
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			log.Debug("K == ", k)
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

func (api *API) walkFiles(pipeline *pipeline.Pipeline, command *pipeline.Command, count *int) {
	log.Debug("Walking ", command.Name, " ", command.ID)
	sources := pipeline.GetPathSources(command)
	available := make(map[string]map[string]string)
	for _, source := range sources {
		var bucket []byte = []byte("files")
		if err := api.Db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket(bucket).Bucket([]byte(pipeline.BucketName))
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
