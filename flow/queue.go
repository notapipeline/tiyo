package flow

import (
	"github.com/choclab-net/tiyo/config"
	"github.com/choclab-net/tiyo/pipeline"

	log "github.com/sirupsen/logrus"
)

type Queue struct {
	QueueBucket    string `default:"queue"`
	PipelineBucket string
	Config         *config.Config
}

func NewQueue(config *config.Config, bucket string) *Queue {
	queue := Queue{
		PipelineBucket: bucket,
		Config:         config,
	}
	return &queue
}

func (queue *Queue) Push(command *pipeline.Command) {
	log.Info("Queuing ", command.Name)

}

/*func (queue *Queue) Pop(container string, pod string) *Command {

}*/
