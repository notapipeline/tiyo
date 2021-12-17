// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/notapipeline/tiyo/pkg/config"
	"github.com/notapipeline/tiyo/pkg/pipeline"
)

// Docker client configuration
// Create docker images for use within TIYO
type Docker struct {
	// Config object containing details of docker repo settings
	Config *config.Config

	// Docker client
	Client *client.Client
}

// ErrorMessage : struct for unpacking errors from the docker client
type ErrorMessage struct {

	// The message retrieved from docker client
	Error string
}

// NewDockerEngine : Create a new docker engine
func NewDockerEngine(config *config.Config) *Docker {
	log.Info("Loading docker engine")
	docker := Docker{}
	docker.Config = config
	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	docker.Client = client
	return &docker
}

// ContainerExists : if pod does not exist, has it previously been built?
// e.g. curl https://registry.hub.docker.com/v1/repositories/choclab/[NAME]/tags
func (docker *Docker) ContainerExists(tag string) (bool, error) {
	parts := strings.Split(tag, ":")
	var (
		err             error
		name            string = parts[0]
		version         string = parts[1]
		apiAddress      string = "https://registry.hub.docker.com/v1/repositories"
		address         string = fmt.Sprintf("%s/%s/tags", apiAddress, name)
		response        *http.Response
		retry, maxretry int = 0, 100
	)

	log.Info("Checking registry for ", name, " ", version)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: docker.Config.UseInsecureTLS,
	}
	http.DefaultClient.Timeout = 10 * time.Second

	// API is the easiest but maybe not the most versatile method of checking
	// This will be potentially very different for artifactory/nexus/quay and
	// it may not be instantly recognisable from the URL that the API is a
	// different endpoint.
	for {
		log.Debug("Making request to ", address)
		response, err = http.Get(address)
		if err, ok := err.(net.Error); ok && err.Timeout() || errors.Is(err, context.DeadlineExceeded) {
			// This is probably temporary and a retry
			// will allow it to succeed.
			log.Info("Context deadline exceeded - will retry in 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		} else if response != nil && response.StatusCode == http.StatusServiceUnavailable {
			if retry >= maxretry {
				return false, err
			}
			retry++
			continue
		} else if err != nil {
			log.Warn("Using default error handler for docker requests")
			return false, err
		}

		if response.StatusCode != 200 {
			return false, nil
		}
		break
	}

	defer response.Body.Close()
	tags := make([]struct {
		Layer string `json:"layer"`
		Name  string `json:"name"`
	}, 0)

	body, err := ioutil.ReadAll(response.Body)
	log.Debug(string(body))
	if err != nil {
		return false, err
	}
	err = json.Unmarshal(body, &tags)
	if err != nil {
		return false, err
	}

	var found bool = false
	for _, v := range tags {
		if v.Name == version {
			found = true
			break
		}
	}
	return found, nil
}

// Build the docker container
func (docker *Docker) build(tag string) error {
	log.Info("Building ", tag)
	buffer := new(bytes.Buffer)
	writer := tar.NewWriter(buffer)
	defer writer.Close()

	files := []string{
		"Dockerfile",
		"tiyo",
		"config.json",
	}

	for _, file := range files {
		reader, err := os.Open(file)
		if err != nil {
			return err
		}

		contents, err := ioutil.ReadAll(reader)
		if err != nil {
			return err
		}
		if file == "Dockerfile" {
			log.Debug(string(contents))
		}

		header := &tar.Header{
			Name: file,
			Size: int64(len(contents)),
		}

		err = writer.WriteHeader(header)
		if err != nil {
			return err
		}
		_, err = writer.Write(contents)
		if err != nil {
			return err
		}
	}

	stream := bytes.NewReader(buffer.Bytes())
	options := types.ImageBuildOptions{
		SuppressOutput: false,
		Context:        stream,
		Dockerfile:     "Dockerfile",
		Remove:         true,
		Tags:           []string{tag},
	}
	log.Debug(options)
	response, err := docker.Client.ImageBuild(context.Background(), stream, options)
	if err != nil {
		return fmt.Errorf("DOCKER: Failed to build container. Message was: %s", err.Error())
	}

	defer response.Body.Close()
	if _, err = io.Copy(os.Stdout, response.Body); err != nil {
		return err
	}

	return nil
}

// Create a new container based off the pipeline element
func (docker *Docker) Create(command *pipeline.Command) error {
	if err := docker.build(command.Tag); err != nil {
		return err
	}

	log.Debug(docker.Config.Docker)
	auth := &types.AuthConfig{
		Username: docker.Config.Docker.Username,
		Password: docker.Config.Docker.Token,
	}
	object, err := json.Marshal(auth)
	if err != nil {
		return err
	}
	encoded := base64.URLEncoding.EncodeToString(object)
	response, err := docker.Client.ImagePush(context.Background(), command.Tag, types.ImagePushOptions{
		RegistryAuth: encoded,
	})
	if err != nil {
		return err
	}
	defer response.Close()

	var message ErrorMessage
	reader := bufio.NewReader(response)
	for {
		stream, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		json.Unmarshal(stream, &message)
		if message.Error != "" {
			return fmt.Errorf(message.Error)
		}
	}

	return nil
}
