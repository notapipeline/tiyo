package fill

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/choclab-net/tiyo/config"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

const (
	MAX_CLIENTS int = 100
)

var requests chan *http.Request = make(chan *http.Request)

type FilledFileEvent struct {
	Filename string
	Bucket   string
	Opened   bool
	Closed   bool
	Deleted  bool
	Config   *config.Config
}

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

func (event *FilledFileEvent) Store() {
	server := event.Config.AssembleServer()
	data := event.JsonBody(event.Bucket, event.Filename)
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

func (event *FilledFileEvent) JsonBody(bucket string, key string) []byte {
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

/**
 * Path filler
 */
type Filler struct {
	Paths  map[string]*FilledFileEvent
	Config *config.Config
}

func NewFiller(config *config.Config) *Filler {
	filler := Filler{}
	filler.Config = config
	filler.Paths = make(map[string]*FilledFileEvent)
	for i := 1; i <= MAX_CLIENTS; i++ {
		go filler.requestMaker()
	}
	return &filler
}

func (filler *Filler) Add(bucket string, path string, notification notify.Event) {
	filename := filepath.Base(path)
	dirname := filepath.Base(filepath.Dir(path))
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

func (event *Filler) requestMaker() {
	var client = &http.Client{}
	var (
		max_retries int = 5
		retries     int = max_retries
		err         error
		response    *http.Response
	)
	for {
		select {
		case request := <-requests:
			for retries > 0 {
				response, err = client.Do(request)
				if err == nil {
					break
				}
				retries -= 1
			}
			if response != nil {
				if err := response.Body.Close(); err != nil {
					log.Println(err)
				}
			}
		}
	}
}
