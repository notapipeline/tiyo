// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"fmt"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

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
	if f := api.pipelineFromContext(c); f == nil {
		log.Error("Not sending pipeline status - failed")
		return
	}
	api.flow.Execute()
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
	api.flow.Stop()
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
	api.flow.Start()
}

// DestroyFlow : Destroys all infrastructure for a given flow
//
// POST /destroyflow
//
// Request parameters:
// - pipeline - The name of the pipeline to trigger
//
func (api *API) DestroyFlow(c *gin.Context) {
	api.flow.Destroy()

	result := Result{
		Code:    200,
		Result:  "OK",
		Message: "OK",
	}

	var (
		err          error
		pipelineName string = api.flow.Pipeline.Name
	)
	for _, name := range []string{"events", "files", "pods", "queue"} {
		if err := api.Db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(name))
			if b == nil {
				return fmt.Errorf("no such bucket")
			}

			if val := b.Get([]byte(pipelineName)); val == nil {
				if err := b.DeleteBucket([]byte(pipelineName)); err != nil {
					return fmt.Errorf("error deleting inner bucket %s - %s", pipelineName, err)
				}
			}

			if err := b.Delete([]byte(pipelineName)); err != nil {
				return fmt.Errorf("error deleting key %s - %s", pipelineName, err)
			}

			if _, err = b.CreateBucketIfNotExists([]byte(pipelineName)); err != nil {
				return fmt.Errorf("error creating inner bucket %s/%s", name, pipelineName)
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

// FlowStatus : Check all infrastructure and get the status of resources
//
// Sends back a map containing information about:
//   - Nodes
//   - Pods
//   - Containers
//   - Sets (groups)
func (api *API) FlowStatus(c *gin.Context) {
	if api.flow == nil {
		if f := api.pipelineFromContext(c); f == nil {
			log.Error("Not sending pipeline status - failed")
			return
		}
	}

	response := make(map[string]interface{})
	response["status"] = "Ready"
	response["groups"] = make(map[string]interface{})
	response["nodes"] = api.flow.Kubernetes.GetNodes()

	var notready = false

	// Get containers from pipeline, then attach build status for each
	groups := make(map[string]interface{})
	for id, container := range api.flow.Pipeline.Containers {
		group := make(map[string]interface{})
		podState, err := api.flow.Kubernetes.PodStatus(strings.Join([]string{api.flow.Pipeline.DNSName, container.Name}, "-"))
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

	result := Result{
		Code:    200,
		Result:  "OK",
		Message: response,
	}
	c.JSON(result.Code, result)
}

// pipelineFromContext : Get the Pipeline instance from the current Gin context
func (api *API) pipelineFromContext(c *gin.Context) *Flow {
	result := Result{
		Code:   400,
		Result: "Error",
	}
	content := make(map[string]string)
	if content["pipeline"] = c.Params.ByName("pipeline"); content["pipeline"] == "" {
		c.ShouldBind(&content)
	}

	if _, ok := content["pipeline"]; !ok {
		log.Error("Pipeline name is required")
		result.Message = "Pipeline name is required"
		c.JSON(result.Code, result)
		return nil
	}

	var (
		pipelineName string = content["pipeline"]
	)

	log.Debug("Finding flow for ", pipelineName)
	if api.flow == nil || api.flow.Pipeline.Name != pipelineName {
		log.Info("Loading new instance of pipeline ", pipelineName)
		newFlow := NewFlow(api)
		newFlow.Config = api.Config

		pipelineJson, err := api.getValue("pipeline", "", pipelineName)
		if err != nil {
			result.Message = "No such pipeline " + pipelineName
			log.Error(result.Message)
			c.JSON(result.Code, result)
			return nil
		}
		if !newFlow.Setup(pipelineName, pipelineJson) {
			log.Error("Failed to configure flow for pipeline ", pipelineName)
			result := Result{
				Code:    500,
				Result:  "Error",
				Message: "Internal server error",
			}
			c.JSON(result.Code, result)
			return nil
		}
		api.flow = newFlow
	}

	return api.flow
}
