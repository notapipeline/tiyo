package flow

import (
	"fmt"
	"os"

	"github.com/choclab-net/tiyo/server"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type FlowApi struct {
	Flow  *Flow
	Bound bool
}

func NewFlowApi(flow *Flow) *FlowApi {
	api := FlowApi{
		Flow:  flow,
		Bound: false,
	}
	return &api
}

func (flowApi *FlowApi) Serve() {
	log.Info("starting flow server - ", flowApi.Flow.Config.FlowServer())
	mode := os.Getenv("TIYO_LOG")
	if mode == "" {
		mode = "production"
	}
	log.Info("Running in ", mode, " mode")
	if mode != "debug" && mode != "trace" {
		gin.SetMode(gin.ReleaseMode)
	}

	var err error
	server := gin.Default()
	server.POST("/api/v1/register", flowApi.Register)
	server.POST("/api/v1/execute", flowApi.Execute)
	server.POST("/api/v1/status", flowApi.Status)
	server.POST("/api/v1/start", flowApi.Start)
	server.POST("/api/v1/stop", flowApi.Stop)
	server.POST("/api/v1/destroy", flowApi.Destroy)

	host := fmt.Sprintf("%s:%d", flowApi.Flow.Config.Flow.Host, flowApi.Flow.Config.Flow.Port)
	log.Info(host)
	if flowApi.Flow.Config.Flow.Cacert != "" && flowApi.Flow.Config.Flow.Cakey != "" {
		err = server.RunTLS(
			host, flowApi.Flow.Config.Flow.Cacert, flowApi.Flow.Config.Flow.Cakey)
	} else {
		err = server.Run(host)
	}

	if err != nil {
		log.Fatal("Cannot run server. ", err)
	}
}

func (flowApi *FlowApi) Register(c *gin.Context) {
	if flowApi.Flow.Queue == nil {
		result := server.Result{
			Code:    404,
			Result:  "Error",
			Message: "Not found - try again later",
		}
		c.JSON(result.Code, result)
		return
	}
	log.Debug("Flow queue = ", flowApi.Flow.Queue)
	flowApi.Flow.Queue.Register(c)
}

func (flowApi *FlowApi) pipelineFromContext(c *gin.Context) bool {
	if flowApi.Bound {
		// already bound
		return true
	}

	result := server.Result{
		Code:   400,
		Result: "Error",
	}
	content := make(map[string]string)
	if err := c.ShouldBind(&content); err != nil {
		log.Error("Pipeline from context ", err)
		result.Message = "Pipeline from context " + err.Error()
		c.JSON(result.Code, result)
		return false
	}
	log.Debug(content)
	flowApi.Bound = true

	if _, ok := content["pipeline"]; !ok {
		log.Error("Pipeline name is required")
		result.Message = "Pipeline name is required"
		c.JSON(result.Code, result)
		return false
	}

	if !flowApi.Flow.IsExecuting {
		flowApi.Flow.Name = content["pipeline"]
		if !flowApi.Flow.Setup(flowApi.Flow.Name) {
			log.Error("Failed to configure flow for pipeline ", flowApi.Flow.Name)
			result := server.Result{
				Code:    500,
				Result:  "Error",
				Message: "Internal server error",
			}
			c.JSON(result.Code, result)
			return false
		}
	}
	return true
}

func (flowApi *FlowApi) Destroy(c *gin.Context) {
	if ok := flowApi.pipelineFromContext(c); !ok {
		log.Error("Not executing pipeline - failed")
		return
	}
	if flowApi.Flow.IsExecuting {
		flowApi.Flow.Stop()
	}
	go flowApi.Flow.Destroy()
	result := server.Result{
		Code:    202,
		Result:  "Accepted",
		Message: "",
	}
	c.JSON(result.Code, result)
}

func (flowApi *FlowApi) Stop(c *gin.Context) {
	if ok := flowApi.pipelineFromContext(c); !ok {
		log.Error("Not executing pipeline - failed")
		return
	}
	if flowApi.Flow.IsExecuting {
		flowApi.Flow.Stop()
	}
	result := server.Result{
		Code:    202,
		Result:  "Accepted",
		Message: "",
	}
	c.JSON(result.Code, result)
}

func (flowApi *FlowApi) Start(c *gin.Context) {
	if ok := flowApi.pipelineFromContext(c); !ok {
		log.Error("Not executing pipeline - failed")
		return
	}
	if !flowApi.Flow.IsExecuting {
		flowApi.Flow.Start()
	}
	result := server.Result{
		Code:    202,
		Result:  "Accepted",
		Message: "",
	}
	c.JSON(result.Code, result)
}

func (flowApi *FlowApi) Execute(c *gin.Context) {
	if ok := flowApi.pipelineFromContext(c); !ok {
		log.Error("Not executing pipeline - failed")
		return
	}

	if !flowApi.Flow.IsExecuting {
		// Execute runs in goroutine to avoid blocking server
		go flowApi.Flow.Execute()
	}
	flowApi.Status(c)
}

// Get the status of the executing pipeline
func (flowApi *FlowApi) Status(c *gin.Context) {
	if ok := flowApi.pipelineFromContext(c); !ok {
		log.Error("Not sending pipeline status - failed")
		return
	}

	response := make(map[string]interface{})
	response["status"] = "Ready"
	response["groups"] = make(map[string]interface{})

	var notready = false

	// Get containers from pipeline, then attach build status for each
	groups := make(map[string]interface{})
	for id, container := range flowApi.Flow.Pipeline.Containers {
		group := make(map[string]interface{})
		podState, err := flowApi.Flow.Kubernetes.PodStatus(container.Name)
		if err != nil {
			log.Error(err)
			continue
		}
		for _, pod := range podState {
			if pod.State == "Executing" {
				response["status"] = "Executing"
			} else if pod.State == "Pending" || pod.State == "Terminated" {
				notready = true
			}
		}

		var equals bool = int32(len(podState)) == container.Scale
		if container.LastCount > len(podState) {
			container.State = "Terminated"
		} else if container.LastCount < len(podState) {
			container.State = "Creating"
		} else if equals {
			container.State = "Ready"
		}
		container.LastCount = len(podState)

		group["state"] = container.State
		group["pods"] = podState
		groups[id] = group
	}
	response["groups"] = groups
	if notready {
		response["status"] = "Creating"
	}

	result := server.Result{
		Code:    200,
		Result:  "OK",
		Message: response,
	}
	c.JSON(result.Code, result)
}
