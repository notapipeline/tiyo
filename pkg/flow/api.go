// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package flow : API subsystem for flow app
package flow

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/notapipeline/tiyo/pkg/config"
	"github.com/notapipeline/tiyo/pkg/server"
	log "github.com/sirupsen/logrus"
)

// API Structure for the flow api
type API struct {

	// The flow API can hold multiple pipelines but only one copy
	// of each individual pipeline.
	// If multiple copies of the same pipeline are required, multiple
	// flow instances need to be executed although this will lead to
	// issues with the current queue implementation
	Instances map[string]Flow

	// Config for the flow api
	config *config.Config
}

// NewAPI Create a new Flow API object
func NewAPI() *API {
	api := API{}
	api.Instances = make(map[string]Flow)
	return &api
}

// Serve the API over HTTP
//
// Flow api offers integrations for the Syphon application to
// communicate and for assemble to trigger the application
// deployments.
func (api *API) Serve(config *config.Config) {
	log.Info("starting flow server - ", config.FlowServer())
	api.config = config

	server := server.NewServer()
	// Used by syphon to regiser a container as ready/busy
	server.Engine.POST("/api/v1/register", api.Register)

	// Execute the pipeline and build infrastructure
	server.Engine.POST("/api/v1/execute", api.Execute)

	// Get the status
	server.Engine.POST("/api/v1/status", api.Status)

	// Start the queue
	server.Engine.POST("/api/v1/start", api.Start)

	// Stop the queue
	server.Engine.POST("/api/v1/stop", api.Stop)

	// destroy all infrastructure related to the pipeline
	server.Engine.POST("/api/v1/destroy", api.Destroy)

	host := fmt.Sprintf("%s:%d", config.Flow.Host, config.Flow.Port)
	log.Info(host)

	var err error
	if config.Flow.Cacert != "" && config.Flow.Cakey != "" {
		err = server.Engine.RunTLS(
			host, config.Flow.Cacert, config.Flow.Cakey)
	} else {
		err = server.Engine.Run(host)
	}

	if err != nil {
		log.Fatal("Cannot run server. ", err)
	}
}

// Register : Endpoint for Syphon executors to register their current status
//
// When the status parameter is sent and is set to ready, if a command is
// available on the queue, this is sent back to syphon.
//
// If syphon registers busy, no command should be sent back.
func (api *API) Register(c *gin.Context) {
	var flow *Flow
	var request map[string]interface{} = api.podRequest(c)
	if request == nil {
		return
	}

	if flow = api.flowFromPodName(request["pod"].(string)); flow == nil || flow.Queue == nil {
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

// podRequest : Unpack a request from a pod and validate the input returning a map containing the verified fields
func (api *API) podRequest(c *gin.Context) map[string]interface{} {
	expected := []string{"pod", "container", "status"}
	request := make(map[string]interface{})
	if err := c.ShouldBind(&request); err != nil {
		for _, expect := range expected {
			request[expect] = c.PostForm(expect)
		}
	}
	if ok, missing := api.checkFields(expected, request); !ok {
		result := server.NewResult()
		result.Code = 400
		result.Result = "Error"
		result.Message = "The following fields are mising from the request " + strings.Join(missing, ", ")
		c.JSON(result.Code, result)
		return nil
	}
	return request
}

// flowFromPodName : Map a pod name back to a flow instance
func (api *API) flowFromPodName(podName string) *Flow {
	for _, flow := range api.Instances {
		if podName == flow.Pipeline.DNSName || strings.HasPrefix(podName, flow.Pipeline.DNSName) {
			return &flow
		}
	}

	log.Info("Flow for ", podName, " not loaded - loading new.")
	var flow *Flow
	if flow = flow.Find(podName, api.config); flow != nil {
		api.Instances[flow.Name] = *flow
		return flow
	}
	return nil
}

// checkFields : Checks a posted request for all expected fields
// return true if fields are ok, false otherwise
func (api *API) checkFields(expected []string, request map[string]interface{}) (bool, []string) {
	log.Debug(request)
	missing := make([]string, 0)
	for _, key := range expected {
		if _, ok := request[key]; !ok {
			missing = append(missing, key)
		}
	}
	return len(missing) == 0, missing
}

// pipelineFromContext : Get the Pipeline instance from the current Gin context
func (api *API) pipelineFromContext(c *gin.Context, rebind bool) *Flow {
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
	if flow, ok = api.Instances[pipelineName]; !ok {
		log.Info("Loading new instance of pipeline ", pipelineName)
		newFlow := NewFlow()
		newFlow.Config = api.config

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
		api.Instances[pipelineName] = *newFlow
		flow = api.Instances[pipelineName]
		flow.Start()
	}

	if rebind {
		log.Debug("Rebinding pipeline ", pipelineName)
		flow.LoadPipeline(pipelineName)
	}

	return &flow
}

// Destroy : Destroy all infrastructure related to the current pipeline
func (api *API) Destroy(c *gin.Context) {
	var flow *Flow
	log.Debug("Destroying pipeline")
	if flow = api.pipelineFromContext(c, false); flow == nil {
		log.Error("Not destroying pipeline - failed")
		return
	}
	if flow.IsExecuting {
		flow.Stop()
	}
	log.Warn("Triggering flow.Destroy()")
	go flow.Destroy()
	result := server.Result{
		Code:    202,
		Result:  "Accepted",
		Message: "",
	}
	c.JSON(result.Code, result)
}

// Stop : Stop events being retrieved from the queue and new events being added
func (api *API) Stop(c *gin.Context) {
	var flow *Flow
	log.Debug("Stopping pipeline")
	if flow = api.pipelineFromContext(c, true); flow == nil {
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

// Start : Allow events to be picked up from the queue and new events to be added
func (api *API) Start(c *gin.Context) {
	var flow *Flow
	log.Debug("Starting pipeline")
	if flow = api.pipelineFromContext(c, true); flow == nil {
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

// Execute : [Re]Create all infrastructure associated with the current pipeline
func (api *API) Execute(c *gin.Context) {
	var flow *Flow
	log.Debug("Executing pipeline")
	if flow = api.pipelineFromContext(c, true); flow == nil {
		log.Error("Not executing pipeline - failed")
		return
	}

	if !flow.IsExecuting {
		// Execute runs in goroutine to avoid blocking server
		go flow.Execute()
	}
	api.checkStatus(c, false)
}

// Status : Get the status of the executing pipeline
func (api *API) Status(c *gin.Context) {
	api.checkStatus(c, true)
}

// checkStatus : Check all infrastructure and get the status of resources
//
// Sends back a map containing information about:
//   - Nodes
//   - Pods
//   - Containers
//   - Sets (groups)
func (api *API) checkStatus(c *gin.Context, rebind bool) {
	var flow *Flow
	if flow = api.pipelineFromContext(c, rebind); flow == nil {
		log.Error("Not sending pipeline status - failed")
		return
	}

	response := make(map[string]interface{})
	response["status"] = "Ready"
	response["groups"] = make(map[string]interface{})
	response["nodes"] = flow.Kubernetes.GetNodes()

	var notready = false

	// Get containers from pipeline, then attach build status for each
	groups := make(map[string]interface{})
	for id, container := range flow.Pipeline.Containers {
		group := make(map[string]interface{})
		podState, err := flow.Kubernetes.PodStatus(strings.Join([]string{flow.Pipeline.DNSName, container.Name}, "-"))
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
