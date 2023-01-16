// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/notapipeline/tiyo/pkg/config"
	"github.com/notapipeline/tiyo/pkg/pipeline"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/scheme"
)

const (
	QPS   = 200
	BURST = 2 * QPS
)

// ContainerStatus : A struct holding the status of a container
type ContainerStatus struct {

	// The JointJS container ID
	ID string `json:"id"`

	// The current state of the container
	State string `json:"state"`

	// If the container has terminated, the reason is placed here
	Reason string `json:"reason"`
}

// PodsStatus : A struct holding the status of a pod
type PodsStatus struct {

	// The status of the pod
	State string `json:"state"`

	// A list of containers held by the pod
	Containers map[string]ContainerStatus `json:"containers"`
}

type LockableApiResource struct {
	sync.RWMutex
	Content map[string][]ApiResource
}

// Kubernetes : Construction struct for Kubernetes
type Kubernetes struct {

	// The kubernetes configuration file for connecting to the cluster
	KubeConfig *rest.Config

	// Kubernetes clientset for connections
	ClientSet *kubernetes.Clientset

	// Rest client for raw api access
	RestClient *rest.RESTClient

	// tiyo config file
	Config *config.Config

	// The pipeline for the current build
	Pipeline *pipeline.Pipeline

	// Api resources as a locked content for periodic reloading
	ApiResources LockableApiResource
}

func contains(what string, where []string) bool {
	for _, v := range where {
		if what == v {
			return true
		}
	}
	return false
}

// NewKubernetes : Create a new Kubernetes engine
func NewKubernetes(config *config.Config, pipeline *pipeline.Pipeline) (*Kubernetes, error) {
	log.Info("Initialising Kubernetes engine")

	kube := Kubernetes{
		Pipeline: pipeline,
		Config:   config,
		ApiResources: LockableApiResource{
			Content: make(map[string][]ApiResource),
		},
	}

	var err error

	if kube.KubeConfig, err = rest.InClusterConfig(); err != nil {
		log.Info("Unable to directly use API, will try local config. %s", err.Error())
		log.Info("Loading config file from ", config.Kubernetes.ConfigFile)
		if kube.KubeConfig, err = clientcmd.BuildConfigFromFlags("", config.Kubernetes.ConfigFile); err != nil {
			return nil, err
		}
	}

	kube.ClientSet, err = kubernetes.NewForConfig(kube.KubeConfig)
	if err != nil {
		return nil, err
	}

	c := *kube.KubeConfig
	c.NegotiatedSerializer = serializer.NewCodecFactory(scheme.Scheme)
	c.UserAgent = rest.DefaultKubernetesUserAgent()
	c.QPS = QPS
	c.Burst = BURST

	if kube.RestClient, err = rest.UnversionedRESTClientFor(&c); err != nil {
		return nil, err
	}

	return &kube, nil
}

// getStateFromDb : Gets any additional reported state from the database
//
// On any error, this will return "running" as this is the default
// state the pod is in.
//
// If no error is detected, the state will be one of "ready"|"busy"
func (kube *Kubernetes) getStateFromDb(podname string, image string) string {
	var slice []string = strings.Split(image, "/")
	var (
		state   string = "Running"
		name    string = slice[len(slice)-1] + ":" + podname
		address        = kube.Config.Address() + "/api/v1/bucket/pods/" + kube.Pipeline.BucketName + "/" + name
	)

	request, err := http.NewRequest("GET", address, nil)
	if err != nil {
		log.Error(err)
		return state
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Connection", "close")
	request.Close = true
	client := &http.Client{
		Timeout: config.TIMEOUT,
	}
	response, err := client.Do(request)
	if err != nil {
		log.Error(err)
		return state
	}
	defer response.Body.Close()
	if response.StatusCode == 200 {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Error(err)
			return state
		}
		var contents struct {
			Message string `json:"message"`
		}
		err = json.Unmarshal(body, &contents)
		if err != nil {
			log.Error(err)
			return state
		}
		state = contents.Message
	}

	return state
}

func (kube *Kubernetes) IsExistingResource(name string) bool {
	return kube.DeploymentExists(name) || kube.StatefulSetExists(name) || kube.DaemonSetExists(name)
}

func (kube *Kubernetes) GetContainerPorts(instance *pipeline.Command) []corev1.ContainerPort {
	ports := make([]corev1.ContainerPort, 0)

	links := kube.Pipeline.GetLinksTo(instance)
	for _, link := range links {
		switch (*link).(type) {
		case *pipeline.PortLink:
			port := corev1.ContainerPort{}
			port.ContainerPort = int32((*link).(*pipeline.PortLink).DestPort)
			port.Protocol = corev1.ProtocolTCP
			if (*link).GetType() == "udp" {
				port.Protocol = corev1.ProtocolUDP
			}
		}
	}
	return ports
}
