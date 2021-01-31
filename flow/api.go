package flow

import (
	"fmt"
	"os"
	"strings"

	"github.com/choclab-net/tiyo/config"
	"github.com/choclab-net/tiyo/server"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type FlowApi struct {
	Instances map[string]Flow
	config    *config.Config
}

type FlowApiInstances struct {
	Flow *Flow
}

func NewFlowApi() *FlowApi {
	api := FlowApi{}
	api.Instances = make(map[string]Flow)
	return &api
}

func (flowApi *FlowApi) Serve(config *config.Config) {
	log.Info("starting flow server - ", config.FlowServer())
	flowApi.config = config
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

	host := fmt.Sprintf("%s:%d", config.Flow.Host, config.Flow.Port)
	log.Info(host)
	if config.Flow.Cacert != "" && config.Flow.Cakey != "" {
		err = server.RunTLS(
			host, config.Flow.Cacert, config.Flow.Cakey)
	} else {
		err = server.Run(host)
	}

	if err != nil {
		log.Fatal("Cannot run server. ", err)
	}
}

func (flowApi *FlowApi) Register(c *gin.Context) {
	var flow *Flow
	var request map[string]interface{} = flowApi.podRequest(c)
	if request == nil {
		return
	}

	if flow = flowApi.flowFromPodName(request["pod"].(string)); flow == nil || flow.Queue == nil {
		result := server.Result{
			Code:    404,
			Result:  "Error",
			Message: "Not found - try again later",
		}
		c.JSON(result.Code, result)
		return
	}
	log.Debug("Flow queue = ", flow.Queue)
	var result *server.Result = flow.Queue.Register(request)
	c.JSON(result.Code, result)
}

func (flowApi *FlowApi) podRequest(c *gin.Context) map[string]interface{} {
	expected := []string{"pod", "container", "status"}
	request := make(map[string]interface{})
	if err := c.ShouldBind(&request); err != nil {
		for _, expect := range expected {
			request[expect] = c.PostForm(expect)
		}
	}
	if ok, missing := flowApi.checkFields(expected, request); !ok {
		result := server.NewResult()
		result.Code = 400
		result.Result = "Error"
		result.Message = "The following fields are mising from the request " + strings.Join(missing, ", ")
		c.JSON(result.Code, result)
		return nil
	}
	return request
}

func (flowApi *FlowApi) flowFromPodName(podName string) *Flow {
	for _, flow := range flowApi.Instances {
		if podName == flow.Pipeline.DnsName || strings.HasPrefix(podName, flow.Pipeline.DnsName) {
			return &flow
		}
	}

	var flow *Flow
	if flow = flow.Find(podName, flowApi.config); flow != nil {
		flowApi.Instances[flow.Name] = *flow
		return flow
	}
	return nil
}

// Checks a posted request for all expected fields
// return true if fields are ok, false otherwise
func (flowApi *FlowApi) checkFields(expected []string, request map[string]interface{}) (bool, []string) {
	log.Debug(request)
	missing := make([]string, 0)
	for _, key := range expected {
		if _, ok := request[key]; !ok {
			missing = append(missing, key)
		}
	}
	return len(missing) == 0, missing
}

func (flowApi *FlowApi) pipelineFromContext(c *gin.Context, rebind bool) *Flow {
	result := server.Result{
		Code:   400,
		Result: "Error",
	}
	content := make(map[string]string)
	if err := c.ShouldBind(&content); err != nil {
		log.Error("Pipeline from context ", err)
		result.Message = "Pipeline from context " + err.Error()
		c.JSON(result.Code, result)
		return nil
	}
	log.Debug(content)

	if _, ok := content["pipeline"]; !ok {
		log.Error("Pipeline name is required")
		result.Message = "Pipeline name is required"
		c.JSON(result.Code, result)
		return nil
	}

	var (
		pipelineName string = content["pipeline"]
		flow         Flow
		ok           bool
	)

	log.Debug("Finding flow for ", pipelineName)
	if flow, ok = flowApi.Instances[pipelineName]; !ok || rebind {
		log.Debug("Setting up Flow due to ", ok, rebind)
		newFlow := NewFlow()
		newFlow.Config = flowApi.config

		if !newFlow.Setup(pipelineName) {
			log.Error("Failed to configure flow for pipeline ", pipelineName)
			result := server.Result{
				Code:    500,
				Result:  "Error",
				Message: "Internal server error",
			}
			c.JSON(result.Code, result)
			return nil
		}
		flowApi.Instances[pipelineName] = *newFlow
		flow = flowApi.Instances[pipelineName]
	}

	return &flow
}

func (flowApi *FlowApi) Destroy(c *gin.Context) {
	var flow *Flow
	log.Debug("Destroying pipeline")
	if flow = flowApi.pipelineFromContext(c, true); flow == nil {
		log.Error("Not destroying pipeline - failed")
		return
	}
	if flow.IsExecuting {
		flow.Stop()
	}
	go flow.Destroy()
	result := server.Result{
		Code:    202,
		Result:  "Accepted",
		Message: "",
	}
	c.JSON(result.Code, result)
}

func (flowApi *FlowApi) Stop(c *gin.Context) {
	var flow *Flow
	log.Debug("Stopping pipeline")
	if flow = flowApi.pipelineFromContext(c, true); flow == nil {
		log.Error("Not stopping pipeline - failed")
		return
	}
	if flow.IsExecuting {
		flow.Stop()
	}
	result := server.Result{
		Code:    202,
		Result:  "Accepted",
		Message: "",
	}
	c.JSON(result.Code, result)
}

func (flowApi *FlowApi) Start(c *gin.Context) {
	var flow *Flow
	log.Debug("Starting pipeline")
	if flow = flowApi.pipelineFromContext(c, true); flow == nil {
		log.Error("Not starting - failed")
		return
	}
	if !flow.IsExecuting {
		flow.Start()
	}
	result := server.Result{
		Code:    202,
		Result:  "Accepted",
		Message: "",
	}
	c.JSON(result.Code, result)
}

func (flowApi *FlowApi) Execute(c *gin.Context) {
	var flow *Flow
	log.Debug("Executing pipeline")
	if flow = flowApi.pipelineFromContext(c, true); flow == nil {
		log.Error("Not executing pipeline - failed")
		return
	}

	if !flow.IsExecuting {
		// Execute runs in goroutine to avoid blocking server
		go flow.Execute()
	}
	flowApi.checkStatus(c, false)
}

// Get the status of the executing pipeline
func (flowApi *FlowApi) Status(c *gin.Context) {
	flowApi.checkStatus(c, true)
}

func (flowApi *FlowApi) checkStatus(c *gin.Context, rebind bool) {
	var flow *Flow
	if flow = flowApi.pipelineFromContext(c, true); flow == nil {
		log.Error("Not sending pipeline status - failed")
		return
	}

	response := make(map[string]interface{})
	response["status"] = "Ready"
	response["groups"] = make(map[string]interface{})

	var notready = false

	// Get containers from pipeline, then attach build status for each
	groups := make(map[string]interface{})
	for id, container := range flow.Pipeline.Containers {
		group := make(map[string]interface{})
		podState, err := flow.Kubernetes.PodStatus(strings.Join([]string{flow.Pipeline.DnsName, container.Name}, "-"))
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
