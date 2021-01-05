package flow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/choclab-net/tiyo/config"
	"github.com/choclab-net/tiyo/pipeline"
	"github.com/choclab-net/tiyo/server"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// we should be able to make this fairly large
const MAX_QUEUE_SIZE = 100000

type QueueItem struct {
	Status    string        `json:"status"` // loading|ready|inprogress|complete|failed
	Pod       string        `json:"pod"`
	Container string        `json:"container"`
	Filename  string        `json:"filename"`
	Event     string        `json:"event"`
	Log       string        `json:"log"`
	Command   SimpleCommand `json:"command"`
}

type Queue struct {
	QueueBucket    string
	FilesBucket    string
	PodBucket      string
	EventsBucket   string
	PipelineBucket string
	Config         *config.Config
	Pipeline       *pipeline.Pipeline
	Client         *http.Client
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
	}
	queue.createBuckets()
	go queue.perpetual()
	return &queue
}

// Register a container into the queue executors
func (queue *Queue) Register(c *gin.Context) {
	expected := []string{"pod", "container", "status"}
	request := make(map[string]interface{})
	if err := c.ShouldBind(&request); err != nil {
		for _, expect := range expected {
			request[expect] = c.PostForm(expect)
		}
	}
	log.Debug(request)
	if ok, missing := queue.checkFields(expected, request); !ok {
		result := server.NewResult()
		result.Code = 400
		result.Result = "Error"
		result.Message = "The following fields are mising from the request " + strings.Join(missing, ", ")
		c.JSON(result.Code, result)
		return
	}

	var key string = request["container"].(string) + ":" + request["pod"].(string)
	data := queue.jsonBody(queue.PodBucket, key, request["status"].(string))
	result := queue.put(c, data)
	if request["status"] == "ready" {
		result.Message = *queue.GetQueueItem(request["container"].(string), request["pod"].(string))
	}
	c.JSON(result.Code, result)
}

// Get a command to execute
func (queue *Queue) GetQueueItem(container string, pod string) *QueueItem {
	server := queue.Config.AssembleServer()

	var key string = container + ":" + pod
	req, err := http.NewRequest(http.MethodGet,
		server+"/api/v1/popqueue/"+queue.QueueBucket+"/"+queue.PipelineBucket+"/"+key, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Accept", "application/json")
	_, body := queue.makeRequest(req)

	item := QueueItem{}
	err = json.Unmarshal(body, &item)
	if err != nil {
		log.Error(err)
		return nil
	}

	return &item
}

// Put data into the bolt store
func (queue *Queue) put(c *gin.Context, request []byte) *server.Result {
	result := server.NewResult()
	result.Code = 204
	result.Result = "No content"
	result.Message = ""

	server := queue.Config.AssembleServer()
	req, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("%s/api/v1/bucket", server),
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

// Checks a posted request for all expected fields
// return true if fields are ok, false otherwise
func (queue *Queue) checkFields(expected []string, request map[string]interface{}) (bool, []string) {
	log.Debug(request)
	missing := make([]string, 0)
	for _, key := range expected {
		if _, ok := request[key]; !ok {
			missing = append(missing, key)
		}
	}
	return len(missing) == 0, missing
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
		return response.StatusCode, nil
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
		server := queue.Config.AssembleServer()
		request, _ := http.NewRequest(http.MethodPost, server+"/api/v1/bucket", bytes.NewBuffer(body))
		request.Header.Set("Content-Type", "application/json; charset=utf-8")
		request.Header.Set("Connection", "close")
		request.Close = true
		queue.makeRequest(request)
	}
}

func (queue *Queue) perpetual() {
	for {
		log.Info("Updating queue for ", queue.Pipeline.Name)
		content := make(map[string]interface{})
		content["pipeline"] = queue.Pipeline.Name
		content["maxitems"] = MAX_QUEUE_SIZE
		data, _ := json.Marshal(content)

		server := queue.Config.AssembleServer()
		request, err := http.NewRequest(
			http.MethodPost,
			server+"/api/v1/perpetualqueue",
			bytes.NewBuffer(data))

		if err != nil {
			log.Error(err)
		}
		request.Header.Set("Content-Type", "application/json; charset=utf-8")
		request.Header.Set("Connection", "close")
		request.Close = true

		response, err := queue.Client.Do(request)
		if err != nil {
			log.Error(err)
		}

		if response.StatusCode != http.StatusAccepted {
			log.Error("Error during processing queue ", response)
		}
		response.Body.Close()
		time.Sleep(10 * time.Second)
	}
}
