// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/notapipeline/tiyo/pkg/config"
	"github.com/notapipeline/tiyo/pkg/pipeline"
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
