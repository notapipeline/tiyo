package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/choclab-net/tiyo/config"
	"github.com/choclab-net/tiyo/pipeline"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var containers []string

type GithubResponse struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Result struct {
	Code    int         `json:"code"`
	Result  string      `json:"result"`
	Message interface{} `json:"message"`
}

type ScanResult struct {
	Buckets []string          `json:"buckets"`
	Keys    map[string]string `json:"keys"`
}

func NewResult() *Result {
	result := Result{}
	return &result
}

type Api struct {
	Db        *bolt.DB
	Config    *config.Config
	QueueSize map[string]int
}

func NewApi(dbName string, config *config.Config) (*Api, error) {
	api := Api{}
	api.Config = config
	var err error
	api.Db, err = bolt.Open(dbName, 0600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}
	api.QueueSize = make(map[string]int)
	return &api, nil
}

func (api *Api) Index(c *gin.Context) {
	c.HTML(200, "index", gin.H{
		"Title": "BoltDB Web Interface",
	})
}

func (api *Api) GetContainers() []string {
	if containers != nil {
		return containers
	}
	request, err := http.NewRequest("GET", "https://api.github.com/repos/BioContainers/containers/contents", nil)
	if err != nil {
		panic(err)
	}
	request.Header.Set("Accept", "application/vnd.github.v3+json")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)

	message := make([]GithubResponse, 0)
	err = json.Unmarshal(body, &message)
	for index := range message {
		if message[index].Type == "dir" {
			containers = append(containers, message[index].Name)
		}
	}
	return containers
}

func (api *Api) Containers(c *gin.Context) {
	result := NewResult()
	result.Code = 200
	result.Result = "OK"
	result.Message = api.GetContainers()
	c.JSON(result.Code, result)
}

func (api *Api) Buckets(c *gin.Context) {

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

func (api *Api) CreateBucket(c *gin.Context) {
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

func (api *Api) DeleteBucket(c *gin.Context) {
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

func (api *Api) DeleteKey(c *gin.Context) {
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

	if request["bucket"] == "" {
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
		fmt.Println(err)
		result.Code = 400
		result.Result = "Error"
		result.Message = err
	}
	if result.Code == 202 {
		c.JSON(result.Code, nil)
	} else {
		c.JSON(result.Code, result)
	}

}

func (api *Api) Put(c *gin.Context) {
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
		result.Code = 400
		result.Result = "Error"
		result.Message = err
	}
	if result.Code == 204 {
		c.JSON(result.Code, nil)
	} else {
		c.JSON(result.Code, result)
	}

}

func (api *Api) Get(c *gin.Context) {
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
		result.Code = 400
		result.Result = "Error"
		result.Message = err
	}
	c.JSON(result.Code, result)

}

func (api *Api) PrefixScan(c *gin.Context) {

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
	if request["key"] == "" {
		request["key"] = request["child"]
		delete(request, "child")
	}
	if request["child"] == "" {
		delete(request, "child")
	}

	scanResults := ScanResult{}
	scanResults.Buckets = make([]string, 0)
	scanResults.Keys = make(map[string]string)

	if err := api.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(request["bucket"]))
		if child, ok := request["child"]; ok {
			b = b.Bucket([]byte(child))
		} else if request["key"] != "" && b.Get([]byte(request["key"])) == nil {
			b = b.Bucket([]byte(request["key"]))
		}

		if b == nil {
			return nil
		}
		c := b.Cursor()

		if key, ok := request["child"]; ok {
			prefix := []byte(key)
			for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
				if v == nil {
					scanResults.Buckets = append(scanResults.Buckets, string(k))
				} else {
					scanResults.Keys[string(k)] = string(v)
				}
			}
		} else {
			for k, v := c.First(); k != nil; k, v = c.Next() {
				if v == nil {
					scanResults.Buckets = append(scanResults.Buckets, string(k))
				} else {
					scanResults.Keys[string(k)] = string(v)
				}
			}
		}
		return nil
	}); err != nil {
		log.Error(err)
		result.Code = 400
		result.Result = "Error"
		result.Message = err.Error()
	}
	result.Message = scanResults
	c.JSON(result.Code, result)

}

func (api *Api) KeyCount(c *gin.Context) {
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
		result.Code = 400
		result.Result = "Error"
		result.Message = err
	}
	c.JSON(result.Code, result)
}

// Updates the given queue with new event items
func (api *Api) PerpetualQueue(c *gin.Context) {
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
	log.Debug("Got pipeline ", pipeline)

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

func (api *Api) walkFiles(pipeline *pipeline.Pipeline, command *pipeline.Command, count *int) {
	log.Debug("Walking ", command.Name, " ", command.Id)
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
				key := tag + ":" + k
				com, _ := json.Marshal(command)
				body := base64.StdEncoding.EncodeToString([]byte(com))

				err := b.Put([]byte(key), []byte(body))
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
				log.Debug("Adding ", command.Id, "to files/", pipeline.BucketName)
				value := available[v]
				value[tag] = "queued"
				com, _ := json.Marshal(command)
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
			api.walkFiles(pipeline, com, count)
		}
	}
}
