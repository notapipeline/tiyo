package flow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"

	"github.com/choclab-net/tiyo/config"
	"github.com/choclab-net/tiyo/pipeline"

	"github.com/choclab-net/tiyo/server"
	log "github.com/sirupsen/logrus"
)

// we should be able to make this fairly large
const MAX_QUEUE_SIZE = 100000

type Queue struct {
	QueueBucket    string
	FilesBucket    string
	PodBucket      string
	EventsBucket   string
	PipelineBucket string
	Config         *config.Config
	Pipeline       *pipeline.Pipeline
	Client         *http.Client
	stopped        bool
}

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
		Client:         &http.Client{},
		stopped:        false,
	}
	queue.createBuckets()
	go queue.perpetual()
	return &queue
}

// TODO: Split this so the api method sits in API and the
// queue management is here.

// Register a container into the queue executors
func (queue *Queue) Register(request map[string]interface{}) *server.Result {
	var key string = request["container"].(string) + ":" + request["pod"].(string)
	log.Debug(queue.PodBucket)
	data := queue.jsonBody(queue.PodBucket, key, request["status"].(string))
	result := queue.put(data)
	if request["status"] == "Ready" {
		var (
			code    int               = 202
			message *server.QueueItem = nil
		)
		if !queue.stopped {
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

func (queue *Queue) Stop() {
	queue.stopped = true
}

func (queue *Queue) Start() {
	queue.stopped = false
}

// Get a command to execute
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

	return code, &item
}

// Put data into the bolt store
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

func (queue *Queue) makeRequest(request *http.Request) (int, []byte) {
	var (
		max_retries int = 5
		retries     int = max_retries
		err         error
		response    *http.Response
		body        []byte
	)
	for retries > 0 {
		response, err = queue.Client.Do(request)
		if err == nil {
			break
		}
		retries -= 1
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

func (queue *Queue) perpetual() {
	var first bool = true
	for {
		if !first {
			time.Sleep(10 * time.Second)
		}
		if queue.stopped {
			continue
		}

		first = false
		log.Info("Updating queue for ", queue.Pipeline.Name)
		content := make(map[string]interface{})
		content["pipeline"] = queue.Pipeline.Name
		content["maxitems"] = MAX_QUEUE_SIZE
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
}
