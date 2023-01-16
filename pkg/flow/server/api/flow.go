// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"flag"

	"github.com/notapipeline/tiyo/pkg/config"
	"github.com/notapipeline/tiyo/pkg/docker"
	kube "github.com/notapipeline/tiyo/pkg/kubernetes"
	"github.com/notapipeline/tiyo/pkg/pipeline"
	log "github.com/sirupsen/logrus"
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
	Docker *docker.Docker

	// Kubernetes engine
	Kubernetes *kube.Kubernetes

	// Flags expected for execution of Flow
	Flags *flag.FlagSet

	// The queue system for managing event handoff from assemble to syphon
	//Queue *Queue

	// Is the pipeline being executed
	IsExecuting bool

	// Is this flow being executed in update mode (will rebuild all containers)
	update bool

	api *API
}

func NewFlow(api *API) *Flow {
	flow := Flow{
		api:    api,
		Config: api.Config,
	}
	flow.IsExecuting = false
	api.flow = &flow
	return api.flow
}

// Setup : Sets up the flow ready for execution
func (flow *Flow) Setup(pipelineName, pipelineJSON string) bool {
	var err error
	log.Println("Setting up flow")

	if flow.Pipeline, err = pipeline.GetPipeline(pipelineJSON, flow.Config); err != nil {
		log.Error("issue loading pipeline ", flow.Pipeline.Name, " - ", err)
		return false
	}
	flow.Pipeline.Name = pipelineName
	flow.Pipeline.DNSName = pipeline.Sanitize(pipelineName, "-")
	flow.Pipeline.Fqdn = flow.Pipeline.DNSName + "." + flow.Config.DNSName

	// Create the queue
	//flow.Queue = NewQueue(flow.Config, flow.Pipeline, flow.Pipeline.BucketName)

	// create docker engine
	flow.Docker = docker.NewDockerEngine(flow.Config)
	if err != nil {
		log.Error(err)
		return false
	}

	// create the Kubernetes engine
	flow.Kubernetes, err = kube.NewKubernetes(flow.Config, flow.Pipeline)
	if err != nil {
		log.Error(err)
		return false
	}
	return true
}

func (flow *Flow) SetupKubernetesOnly() {
	p := pipeline.Pipeline{}
	var err error
	log.Info("Initialising kubernetes client")
	flow.Kubernetes, err = kube.NewKubernetes(flow.Config, &p)
	if err != nil {
		log.Error(err)
		return
	}
	go flow.Kubernetes.ApiDiscovery()
}

// Stop : Stop the flow queue from executing - does not stop the build
func (flow *Flow) Stop() {
	flow.IsExecuting = false
	//flow.Queue.Stop()
}

// Start : Starts the flow queue
func (flow *Flow) Start() {
	flow.IsExecuting = true
	//flow.Queue.Start()
}

// Destroy : Destroys any infrastructure relating to this flow
func (flow *Flow) Destroy() {
	flow.Stop()
	log.Warn("Destroying flow for ", flow.Pipeline.Name)
	for _, item := range flow.Pipeline.Controllers {
		switch item.SourceType {
		case "statefulset":
			go flow.Kubernetes.DestroyStatefulSet(flow.Pipeline.DNSName, item)
		case "deployment":
			go flow.Kubernetes.DestroyDeployment(flow.Pipeline.DNSName, item)
		case "daemonset":
			go flow.Kubernetes.DestroyDaemonSet(flow.Pipeline.DNSName, item)
		}
	}
}

// Execute : Triggers the current flow building all components
func (flow *Flow) Execute() {
	flow.Start()
	// create all missing resources
	for _, command := range flow.Pipeline.Commands {
		log.Debug("Pipeline start item", command)
		err := flow.Create(command)
		if err != nil {
			log.Error(err)
			return
		}
	}

	// PVs need to exist before anything else

	// Create the pipeline runtime engine
	// Each of these needs a level of error reporting enabling
	// other than "panic"
	for _, item := range flow.Pipeline.Controllers {
		switch item.SourceType {
		case "statefulset":
			log.Info("Launching create statefulset")
			go flow.Kubernetes.CreateStatefulSet(flow.Pipeline.DNSName, item)
		case "deployment":
			go flow.Kubernetes.CreateDeployment(flow.Pipeline.DNSName, item)
		case "daemonset":
			go flow.Kubernetes.CreateDaemonSet(flow.Pipeline.DNSName, item)
		}

		go flow.checkout(item.GetChildren())
	}
	//flow.triggerServices()
}
