// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/notapipeline/tiyo/pkg/config"
)

// Containers : The API endpoint method for retrieving the sidebar container set
//
// Returned responses will be one of:
// - 200 OK    : Message will be a list of strings
func (api *API) Containers(c *gin.Context) {
	result := NewResult()
	result.Code = 200
	result.Result = "OK"
	result.Message = api.containers()
	c.JSON(result.Code, result)
}

// containers : Get the list of containers for the sidebar on the pipeline page
func (api *API) containers() []string {
	if containers != nil {
		return containers
	}
	request, err := http.NewRequest("GET", "https://api.github.com/repos/BioContainers/containers/contents", nil)
	if err != nil {
		panic(err)
	}
	request.Header.Set("Accept", "application/vnd.github.v3+json")
	client := &http.Client{
		Timeout: config.TIMEOUT,
	}
	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	body, _ := ioutil.ReadAll(response.Body)

	message := make([]GithubResponse, 0)
	json.Unmarshal(body, &message)
	for index := range message {
		if message[index].Type == "dir" {
			containers = append(containers, message[index].Name)
		}
	}
	return containers
}
