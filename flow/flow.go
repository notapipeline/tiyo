// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package flow : This is the main executor for the pipeline sub-system
// Flow takes the pipeline object, iterates over it and builds the
// elements contained within it. It also contains handlers for
// handing status information back to assemble front end, and for
// delivering commands into the syphon executors via API calls.
//
// Flow can be triggered in single pipeline mode by specifying the pipeline
// as a command line parameter (-p), and/or in update mode which will cause
// any containers in the pipeline to be rebuilt - useful for updating Tiyo
// inside any containers active in the pipeline.
package flow

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/notapipeline/tiyo/config"
	"github.com/notapipeline/tiyo/pipeline"
	"github.com/notapipeline/tiyo/server"
)

// Flow : Main structure of the Flow subsystem
type Flow struct {

	// The real name (non-formatted) of the pipeline used by Flow
	Name string

	// The config system used by flow
	Config *config.Config

	// The pipeline executed by this instance of flow
	Pipeline *pipeline.Pipeline

	// The docker engine
	Docker *Docker

	// Kubernetes engine
	Kubernetes *Kubernetes

	// Flags expected for execution of Flow
	Flags *flag.FlagSet

	// The queue system for managing event handoff from assemble to syphon
	Queue *Queue

	// Is the pipeline being executed
	IsExecuting bool

	// The API subsystem
	API *API

	// Is this flow being executed in update mode (will rebuild all containers)
	update bool
}

// NewFlow : Construct a new Flow object
func NewFlow() *Flow {
	flow := Flow{}
	flow.API = NewAPI()
	flow.IsExecuting = false
	return &flow
}

// Init : Parse the command line and any environment variables for configuring Flow
func (flow *Flow) Init() {
	log.Info("Initialising flow")
	flow.Name = os.Getenv("TIYO_PIPELINE")
	description := "The name of the pipeline to use"
	flow.Flags = flag.NewFlagSet("flow", flag.ExitOnError)
	flow.Flags.StringVar(&flow.Name, "p", flow.Name, description)
	flow.Flags.BoolVar(&flow.update, "u", false, "Update any containers")
	flow.Flags.Parse(os.Args[2:])
	log.Debug("Flow initialised", flow)
}

// Create : Creates a new docker container image if one is not already found in the library
func (flow *Flow) Create(instance *pipeline.Command) error {
	log.Info("flow - Creating new container instance for ", instance.Name, " ", instance.ID)
	var err error
	var containerExists bool
	containerExists, err = flow.Docker.ContainerExists(instance.Tag)
	if err != nil {
		return err
	}

	if containerExists && !flow.update {
		log.Info("Not building image for ", instance.Image, " Image exists")
		return nil
	}

	path := fmt.Sprintf("containers/%s", instance.Tag)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create container build directory and CD to it
		owd, _ := os.Getwd()
		os.MkdirAll(path, 0775)
		os.Chdir(path)
		log.Debug("Changing to build path", path)
		if err := flow.WriteDockerfile(instance); err != nil {
			return flow.Cleanup(path, owd, err)
		}

		if err := flow.CopyTiyoBinary(); err != nil {
			return flow.Cleanup(path, owd, err)
		}

		if err := flow.WriteConfig(); err != nil {
			return flow.Cleanup(path, owd, err)
		}
		err = flow.Docker.Create(instance)
		if err != nil {
			return flow.Cleanup(path, owd, err)
		}
		flow.Cleanup(path, owd, nil)
	}
	return nil
}

// Cleanup : Delete any redundant files left over from building the container
func (flow *Flow) Cleanup(path string, owd string, err error) error {
	os.Chdir(owd)
	if e := os.RemoveAll(path); e != nil {
		log.Error("Failed to clean up %s - manual intervention required\n", path)
	}
	return err
}

// WriteDockerfile ; Writes the template dockerfile ready for building the container
func (flow *Flow) WriteDockerfile(instance *pipeline.Command) error {
	log.Info("Creating Dockerfile ", instance.Image)
	var name string = "Dockerfile"
	template := fmt.Sprintf(dockerTemplate, instance.Image)
	if instance.Language == "dockerfile" && instance.Custom {
		var (
			script []byte
			err    error
		)
		if script, err = base64.StdEncoding.DecodeString(instance.ScriptContent); err != nil {
			return err
		}
		template = string(script)
	}

	file, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("Failed to create Dockerfile for %s. %s", instance.Name, err)
	}
	defer file.Close()
	if _, err := file.WriteString(template); err != nil {
		return fmt.Errorf("Failed to write Dockerfile for %s. Error was: %s", name, err)
	}
	file.Sync()
	log.Debug("Dockerfile written: ", instance.Image)
	return nil
}

// CopyTiyoBinary : Tiyo embeds itself into the containers it build to run in Syphon mode.
func (flow *Flow) CopyTiyoBinary() error {
	log.Debug("Copying tiyo binary")

	path, err := os.Executable()
	if err != nil {
		return err
	}
	sourceFileStat, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}

	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(filepath.Base(path))
	if err != nil {
		return err
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return err
	}
	return nil
}

// WriteConfig : Create a basic config for Syphon to communicate with the current flow
func (flow *Flow) WriteConfig() error {
	log.Debug("Creating stub config for container wrap")
	path, _ := os.Getwd()
	config := struct {
		SequenceBaseDir string      `json:"sequenceBaseDir"`
		UseInsecureTLS  bool        `json:"skipVerify"`
		Flow            config.Host `json:"flow"`
		AppName         string      `json:"appname"`
	}{
		SequenceBaseDir: flow.Config.SequenceBaseDir,
		UseInsecureTLS:  flow.Config.UseInsecureTLS,
		Flow:            flow.Config.Flow,
		AppName:         filepath.Base(path),
	}
	bytes, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile("config.json", bytes, 0644); err != nil {
		return err
	}
	return nil
}

// Setup : Sets up the flow ready for execution
func (flow *Flow) Setup(pipelineName string) bool {
	flow.Name = pipelineName
	var err error

	if !flow.LoadPipeline(pipelineName) {
		return false
	}
	// Create the queue
	flow.Queue = NewQueue(flow.Config, flow.Pipeline, flow.Pipeline.BucketName)

	// create docker engine
	flow.Docker = NewDockerEngine(flow.Config)
	if err != nil {
		log.Error(err)
		return false
	}

	// create the Kubernetes engine
	flow.Kubernetes, err = NewKubernetes(flow.Config, flow.Pipeline)
	if err != nil {
		log.Error(err)
		return false
	}
	return true
}

// LoadPipeline : Loads a pipeline from a given name returning false if the pipeline fails to load
func (flow *Flow) LoadPipeline(pipelineName string) bool {
	// Load the pipeline
	var err error
	flow.Pipeline, err = pipeline.GetPipeline(flow.Config, flow.Name)
	if err != nil {
		log.Error("issue loading pipeline ", flow.Name, " - ", err)
		return false
	}
	return true
}

// Execute : Triggers the current flow building all components
func (flow *Flow) Execute() {
	flow.Start()
	// create all missing containers
	for _, command := range flow.Pipeline.Commands {
		log.Debug("Pipeline start item", command)
		err := flow.Create(command)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Create the pipeline runtime engine
	// Each of these needs a level of error reporting enabling
	// other than "panic"
	for _, item := range flow.Pipeline.Containers {
		switch item.SetType {
		case "statefulset":
			go flow.Kubernetes.CreateStatefulSet(flow.Pipeline.DNSName, item)
		case "deployment":
			go flow.Kubernetes.CreateDeployment(flow.Pipeline.DNSName, item)
		case "daemonset":
			go flow.Kubernetes.CreateDaemonSet(flow.Pipeline.DNSName, item)
		}

		go flow.checkout(item.GetChildren())
	}
	flow.triggerServices()
}

// Find : Find a pipeline by name and return a new Flow object with the pipeline embedded
func (flow *Flow) Find(name string, config *config.Config) *Flow {
	log.Debug("Searching for pipeline matching ", name)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: config.UseInsecureTLS,
	}

	// really wants to be a "keys" list rather than a full scan
	var url string = fmt.Sprintf("%s/api/v1/scan/pipeline", config.AssembleServer())
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error(err)
		return nil
	}

	client := &http.Client{
		Timeout: config.TIMEOUT,
	}
	response, err := client.Do(request)
	if err != nil {
		log.Error(err)
		return nil
	}

	defer response.Body.Close()
	message := struct {
		Code    int                    `json:"code"`
		Result  string                 `json:"result"`
		Message map[string]interface{} `json:"message"`
	}{}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Error(err)
		return nil
	}
	err = json.Unmarshal(body, &message)
	if err != nil {
		log.Error(err)
		return nil
	}

	var newFlow *Flow = NewFlow()
	newFlow.Config = config
	for key := range message.Message["keys"].(map[string]interface{}) {
		log.Debug(key)
		var pipelineName string = pipeline.Sanitize(key, "-")
		log.Debug(pipelineName, " ", name)
		if pipelineName == name || strings.HasPrefix(name, pipelineName) {
			newFlow.Setup(key)
			return newFlow
		}
	}
	return nil
}

// Stop : Stop the flow queue from executing - does not stop the build
func (flow *Flow) Stop() {
	flow.IsExecuting = false
	flow.Queue.Stop()
}

// Start : Starts the flow queue
func (flow *Flow) Start() {
	flow.IsExecuting = true
	flow.Queue.Start()
}

// Destroy : Destroys any infrastructure relating to this flow
func (flow *Flow) Destroy() {
	flow.Stop()
	log.Warn("Destroying flow for ", flow.Pipeline.Name)
	for _, item := range flow.Pipeline.Containers {
		switch item.SetType {
		case "statefulset":
			go flow.Kubernetes.DestroyStatefulSet(flow.Pipeline.DNSName, item)
		case "deployment":
			go flow.Kubernetes.DestroyDeployment(flow.Pipeline.DNSName, item)
		case "daemonset":
			go flow.Kubernetes.DestroyDaemonSet(flow.Pipeline.DNSName, item)
		}
	}
}

// Run : Run Flow
func (flow *Flow) Run() int {
	var (
		err error
	)
	log.Info("Starting flow executor")

	sigc := make(chan os.Signal, 1)
	done := make(chan bool)

	signal.Notify(sigc, os.Interrupt)
	go func() {
		for range sigc {
			log.Info("Shutting down listener")
			done <- true
		}
	}()

	flow.Config, err = config.NewConfig()
	if err != nil {
		log.Error("issue loading config file: ", err)
		return 1
	}

	log.Info("Setting working directory to ", flow.Config.DbDir)
	os.Chdir(flow.Config.DbDir)
	// Start server in background
	go flow.API.Serve(flow.Config)
	if flow.Name != "" {
		flow.Setup(flow.Name)
		flow.Execute()
	}
	<-done
	log.Info("Flow complete")
	return 0
}

// triggerServices : If any containers in the pipeline have been defined as auto-start or
// they directly expose ports, these commands should be automatically executed and not wait
// for events to stream in from outside.
//
// This is achieved by adding an iniial command to the queue destined for those services.
//
// TODO: Needs scale factor adding - should be one instance of command per pod.
func (flow *Flow) triggerServices() {
	for _, container := range flow.Pipeline.Containers {
		for _, instance := range container.GetChildren() {
			var execute bool = instance.AutoStart
			if instance.ExposePort > 0 {
				execute = true
			}

			if !execute {
				continue
			}

			log.Info("Triggering service command for ", instance.Name)
			// example:
			// rna-star-tiyo:2.7.7a:sorting:root:GL53_003_Plate3_c1_Gfi1_HE_S3 - command.ID
			var commandKey string = instance.GetContainer(true) + ":" + container.Name
			var contents map[string]string = map[string]string{
				"bucket": "queue",
				"child":  flow.Pipeline.BucketName,
				"key":    commandKey,
				"value":  instance.ID,
			}
			data, _ := json.Marshal(contents)

			var address string = flow.Config.AssembleServer() + "/api/v1/bucket"
			request, err := http.NewRequest(
				http.MethodPut,
				address,
				bytes.NewBuffer(data))

			if err != nil {
				log.Error(err)
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
			} else if response.StatusCode > 204 {
				log.Error("Received unknown response code ", response.StatusCode, " for address ", address)
			}
			log.Info("Queued command ", commandKey)
		}
	}
}

type DecryptBody struct {
	Value string `json:"value"`
	Token string `json:"token"`
}

// Decrypt : Sends a string back to assemble for decryption
func (flow *Flow) Decrypt(what string) (string, error) {
	var message string = ""
	var content DecryptBody = DecryptBody{
		Value: what,
		Token: flow.Config.GetPassphrase("flow"),
	}
	data, _ := json.Marshal(content)

	var address string = flow.Config.AssembleServer() + "/api/v1/decrypt"
	request, err := http.NewRequest(
		http.MethodPost,
		address,
		bytes.NewBuffer(data))

	if err != nil {
		return message, err
	}

	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Connection", "close")
	request.Close = true

	client := &http.Client{
		Timeout: config.TIMEOUT,
	}
	response, err := client.Do(request)
	if err != nil {
		return message, err
	} else if response.StatusCode != 200 {
		return message, fmt.Errorf("Failed to decrypt password - %d ", response.StatusCode)
	}

	var body []byte
	defer response.Body.Close()
	body, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return message, err
	}

	result := server.Result{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return message, err
	}
	return result.Message.(string), nil
}

// Clones a git repository for each container in the set that has a git repo described
func (flow *Flow) checkout(containers []*pipeline.Command) {
	for _, container := range containers {
		var path string = filepath.Join(
			flow.Config.SequenceBaseDir,
			flow.Config.Kubernetes.Volume,
			container.Name,
		)
		var password string = container.GitRepo.Password
		if password == "" {
			if container.GitRepo.Username != "" {
				if _, ok := flow.Pipeline.Credentials[container.GitRepo.Username]; !ok {
					log.Error("No password supplied for repo ", container.GitRepo.RepoURL)
					return
				}
				password = flow.Pipeline.Credentials[container.GitRepo.Username]
			}
		}
		// There is no need to decrypt the password until it is requried.
		// This aids in keeping the app secure by not holding unencrypted
		// passwords in memory for longer than they absolutely need to be.

		if password != "" {
			passwordDecrypted, err := flow.Decrypt(password)
			if err != nil {
				log.Error(err)
			}

			var options map[string]string = map[string]string{
				"password": passwordDecrypted,
			}

			container.GitRepo.Clone(path, options)
			container.GitRepo.Checkout()
		}
	}
}
