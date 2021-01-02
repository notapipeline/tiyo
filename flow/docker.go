package flow

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/choclab-net/tiyo/config"
	"github.com/choclab-net/tiyo/pipeline"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type Docker struct {
	Config *config.Config
	Client *client.Client
}

type ErrorMessage struct {
	Error string
}

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

// if pod does not exist, has it previously been built?
// e.g. curl https://registry.hub.docker.com/v1/repositories/choclab/[NAME]/tags
func (docker *Docker) ContainerExists(tag string) (bool, error) {
	parts := strings.Split(tag, ":")
	var name string = parts[0]
	var version string = parts[1]
	log.Info("Checking registry for ", name, " ", version)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: docker.Config.UseInsecureTLS,
	}

	// API is the easiest but maybe not the most versatile method of checking
	// This will be potentially very different for artifactory/nexus/quay and
	// it may not be instantly recognisable from the URL that the API is a
	// different endpoint.
	var apiAddress string = "https://registry.hub.docker.com/v1/repositories"
	var address string = fmt.Sprintf("%s/%s/tags", apiAddress, name)
	log.Debug("Making request to ", address)
	response, err := http.Get(address)
	if err != nil {
		return false, err
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

func (docker *Docker) Build(tag string) error {
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
		return err
	}

	defer response.Body.Close()
	if _, err = io.Copy(os.Stdout, response.Body); err != nil {
		return err
	}

	return nil
}

func (docker *Docker) Create(command *pipeline.Command) error {
	// Tag the new image
	/*if err := docker.Client.ImageTag(context.Background(), image, tag); err != nil {
		return err
	}*/
	if err := docker.Build(command.Image); err != nil {
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
	response, err := docker.Client.ImagePush(context.Background(), command.Image, types.ImagePushOptions{
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
