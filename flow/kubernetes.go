// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/notapipeline/tiyo/config"
	"github.com/notapipeline/tiyo/pipeline"
	"github.com/notapipeline/tiyo/server"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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

// Kubernetes : Construction struct for Kubernetes
type Kubernetes struct {

	// The kubernetes configuration file for connecting to the cluster
	KubeConfig *rest.Config

	// Kubernetes clientset for connections
	ClientSet *kubernetes.Clientset

	// tiyo config file
	Config *config.Config

	// The pipeline for the current build
	Pipeline *pipeline.Pipeline
}

// NewKubernetes : Create a new Kubernetes engine
func NewKubernetes(config *config.Config, pipeline *pipeline.Pipeline) (*Kubernetes, error) {
	log.Info("Initialising Kubernetes engine")
	kube := Kubernetes{
		Pipeline: pipeline,
		Config:   config,
	}

	var err error

	log.Info("Loading config file from ", config.Kubernetes.ConfigFile)
	kube.KubeConfig, err = clientcmd.BuildConfigFromFlags("", config.Kubernetes.ConfigFile)
	if err != nil {
		return nil, err
	}

	kube.ClientSet, err = kubernetes.NewForConfig(kube.KubeConfig)
	if err != nil {
		return nil, err
	}

	return &kube, nil
}

// NodeMetrics : Information about nodes
type NodeMetrics struct {

	// the max cpu capacity of this node in millicpus
	CPUCapacity int64 `json:"cpucapacity"`

	// the number of requests on this node in millicpus
	CPURequests int64 `json:"cpurequests"`

	// the cpu limits on this node in millicpus
	CPULimits int64 `json:"cpulimits"`

	// the amount of memory available to this node in bytes
	MemoryCapacity int64 `json:"memorycapacity"`

	// the memory requests on this node in bytes
	MemoryRequests int64 `json:"memoryrequests"`

	// the memory limits on this node in bytes
	MemoryLimits int64 `json:"memorylimits"`
}

// GetNodes : Get all nodes in the kubernetes cluster as a map[string]*NodeMetrics
func (kube *Kubernetes) GetNodes() map[string]*NodeMetrics {
	nodes := make(map[string]*NodeMetrics)
	list, err := kube.ClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error("Cannot retrieve node list ", err)
	}
	for _, node := range list.Items {
		nodes[node.Name] = kube.GetNodeResources(&node)
	}

	return nodes
}

// GetNodeResources : Get the resources available to a node
func (kube *Kubernetes) GetNodeResources(node *corev1.Node) *NodeMetrics {
	nodeMetrics := NodeMetrics{
		CPUCapacity:    node.Status.Capacity.Cpu().MilliValue(),
		CPURequests:    0,
		CPULimits:      0,
		MemoryCapacity: node.Status.Capacity.Memory().MilliValue(),
		MemoryRequests: 0,
		MemoryLimits:   0,
	}

	pods, err := kube.ClientSet.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + node.Name,
	})
	if err != nil {
		log.Error("Failed to retrieve pods for node " + node.Name)
	}
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			requests := container.Resources.Requests
			limits := container.Resources.Limits
			for name, value := range requests {
				switch name {
				case corev1.ResourceCPU:
					nodeMetrics.CPURequests += int64(value.MilliValue())
				case corev1.ResourceMemory:
					nodeMetrics.MemoryRequests += int64(value.MilliValue())
				}
			}
			for name, value := range limits {
				switch name {
				case corev1.ResourceCPU:
					nodeMetrics.CPURequests += int64(value.MilliValue())
				case corev1.ResourceMemory:
					nodeMetrics.MemoryRequests += int64(value.MilliValue())
				}
			}
		}
	}

	return &nodeMetrics
}

// GetExternalNodeIPs : Get the list of external IPs available to the cluster
func (kube *Kubernetes) GetExternalNodeIPs() []string {
	// TODO: filter out master
	list, err := kube.ClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error("Cannot retrieve node list ", err)
	}
	log.Debug("Found ", len(list.Items), " nodes ")

	addresses := make([]string, 0)
	for _, node := range list.Items {
		log.Debug("Node addresses ", node.Status.Addresses)
		for _, addr := range node.Status.Addresses {
			if addr.Type == "InternalIP" {
				addresses = append(addresses, addr.Address)
			}
		}
	}
	log.Debug("Found external node addresses ", addresses)
	return addresses
}

// GetPod : Get a single pod by name
func (kube *Kubernetes) GetPod(name string) *corev1.Pod {
	pod, _ := kube.ClientSet.CoreV1().Pods(kube.Config.Kubernetes.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})
	return pod
}

// PodStatus : Get the status of a pod and its containers
func (kube *Kubernetes) PodStatus(name string) (map[string]PodsStatus, error) {
	pods, err := kube.ClientSet.CoreV1().Pods(kube.Config.Kubernetes.Namespace).List(
		context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", name),
		})
	if err != nil {
		return nil, err
	}
	log.Debug("Searching for ", name, " returned ", len(pods.Items), " pods")

	statuses := make(map[string]PodsStatus)
	for _, pod := range pods.Items {
		podStatus := PodsStatus{}
		podStatus.State = string(pod.Status.Phase)

		cstatus := make(map[string]ContainerStatus)
		for _, container := range pod.Status.ContainerStatuses {
			image := container.Name
			var (
				state  string = "Waiting"
				reason string = ""
			)
			if container.State.Running != nil {
				state = kube.getStateFromDb(pod.Name, image)
				// if any container is busy, pod state is executing
				if state == "Busy" {
					podStatus.State = "Executing"
				}
			} else if container.State.Terminated != nil {
				state = "Terminated"
				reason = container.State.Terminated.Reason
			}
			if _, ok := cstatus[image]; !ok {
				var c *pipeline.Command = kube.Pipeline.CommandFromContainerName(name, image)
				var id string = ""
				if c != nil {
					id = c.ID
				}
				cstatus[image] = ContainerStatus{
					ID:     id,
					State:  state,
					Reason: reason,
				}
			}
		}
		podStatus.Containers = cstatus
		statuses[pod.Name] = podStatus
	}
	return statuses, nil
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
		address        = kube.Config.AssembleServer() + "/api/v1/bucket/pods/" + kube.Pipeline.BucketName + "/" + name
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
		contents := server.Result{}
		err = json.Unmarshal(body, &contents)
		if err != nil {
			log.Error(err)
			return state
		}
		state = string(contents.Message.(string))
	}

	return state
}

// PodExists : Check that a pod exists
func (kube *Kubernetes) PodExists(name string) bool {
	list, err := kube.ClientSet.CoreV1().Pods(kube.Config.Kubernetes.Namespace).List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error(err)
	}

	for _, item := range list.Items {
		if item.Name == name {
			return true
		}
	}
	return false
}

// STATEFUL SET METHODS

// GetStatefulSet : Retrieves a statefulset configuration from the cluster
func (kube *Kubernetes) GetStatefulSet(name string) *appsv1.StatefulSet {
	statefulset, _ := kube.ClientSet.AppsV1().StatefulSets(kube.Config.Kubernetes.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})
	return statefulset
}

// StatefulSetExists : Check that a statefulset of a given name exists
func (kube *Kubernetes) StatefulSetExists(name string) bool {
	list, err := kube.ClientSet.AppsV1().StatefulSets(kube.Config.Kubernetes.Namespace).List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error(err)
	}

	for _, item := range list.Items {
		if item.Name == name {
			return true
		}
	}
	return false
}

// GetStatefulSetContainers : Get all containers under a statefulset
func (kube *Kubernetes) GetStatefulSetContainers(instances []*pipeline.Command) []corev1.Container {
	containers := make([]corev1.Container, 0)
	for _, instance := range instances {
		container := corev1.Container{
			Name:            instance.Name,
			Image:           instance.Tag,
			ImagePullPolicy: corev1.PullAlways,
			Ports:           kube.GetContainerPorts(instance),
			VolumeMounts:    kube.GetVolumeMountForNamespace(kube.Config.Kubernetes.Namespace),
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					// hard coded for the moment but should come from form
					"cpu":    resource.MustParse(instance.CPU),
					"memory": resource.MustParse(instance.Memory),
				},
			},
		}
		containers = append(containers, container)
	}
	return containers
}

// DestroyStatefulSet : Destroys a given statefulset and all resources under it
func (kube *Kubernetes) DestroyStatefulSet(pipeline string, container *pipeline.Container) {
	var name string = pipeline + "-" + container.Name
	log.Info("Deleting stateful set ", name)
	client := kube.ClientSet.AppsV1().StatefulSets(kube.Config.Kubernetes.Namespace)

	policy := metav1.DeletePropagationForeground
	client.Delete(context.TODO(), name, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})

	for {
		if !kube.StatefulSetExists(name) {
			log.Info("Deleted statefulset ", name)
			break
		}
		container.State = "Terminating"
		log.Info("Still deleting statefulset ", name)
		time.Sleep(1 * time.Second)
	}

	if kube.ServiceExists(name) {
		kube.DestroyService(name)
	}
}

// CreateStatefulSet : Create a new statefulset
func (kube *Kubernetes) CreateStatefulSet(pipeline string, container *pipeline.Container) {
	var name string = pipeline + "-" + container.Name
	client := kube.ClientSet.AppsV1().StatefulSets(kube.Config.Kubernetes.Namespace)
	instances := container.GetChildren()

	// If the stateful set exists, delete it so it can be re-created.
	if kube.StatefulSetExists(name) {
		container.State = "Terminating"
		kube.DestroyStatefulSet(pipeline, container)
	}

	log.Info("Creating stateful set ", name)
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: kube.Config.Kubernetes.Namespace,
			Labels: map[string]string{
				"app": pipeline,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &container.Scale,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			ServiceName: kube.CreateService(name, instances),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: kube.GetStatefulSetContainers(instances),
					Volumes: []corev1.Volume{
						{
							Name: kube.Config.Kubernetes.Volume,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: kube.Config.Kubernetes.Volume,
								},
							},
						},
					},
				},
			},
		},
	}

	// otherwise create
	result, err := client.Create(context.TODO(), statefulset, metav1.CreateOptions{})
	if err != nil {
		log.Panic(err)
	}
	container.State = "Creating"
	log.Debug(result)
	log.Info("Created stateful set ", result.GetObjectMeta().GetName())
}

// DAEMON SET METHODS

func (kube *Kubernetes) GetDaemonSet(name string) *appsv1.DaemonSet {
	daemonset, _ := kube.ClientSet.AppsV1().DaemonSets(kube.Config.Kubernetes.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})
	return daemonset
}

func (kube *Kubernetes) DaemonSetExists(name string) bool {
	list, err := kube.ClientSet.AppsV1().DaemonSets(kube.Config.Kubernetes.Namespace).List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error(err)
	}

	for _, item := range list.Items {
		if item.Name == name {
			return true
		}
	}
	return false
}

func (kube *Kubernetes) DestroyDaemonSet(pipeline string, container *pipeline.Container) {
	var name string = pipeline + "-" + container.Name
	log.Info("Deleting DaemonSet ", name)
	client := kube.ClientSet.AppsV1().DaemonSets(kube.Config.Kubernetes.Namespace)
	policy := metav1.DeletePropagationForeground
	client.Delete(context.TODO(), name, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})

	for {
		if !kube.DaemonSetExists(name) {
			log.Info("Deleted daemonset ", name)
			break
		}
		log.Info("Still deleting daemonset ", name)
		time.Sleep(1 * time.Second)
	}

	if kube.ServiceExists(name) {
		kube.DestroyService(name)
	}
}

func (kube *Kubernetes) CreateDaemonSet(pipeline string, container *pipeline.Container) {
	var name string = pipeline + "-" + container.Name
	client := kube.ClientSet.AppsV1().DaemonSets(kube.Config.Kubernetes.Namespace)

	// If the daemonset exists, delete and recreate it.
	if kube.DaemonSetExists(name) {
		container.State = "Terminating"
		kube.DestroyDaemonSet(pipeline, container)
	}

	daemonset := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: kube.Config.Kubernetes.Namespace,
			Labels: map[string]string{
				"app": pipeline,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: kube.GetStatefulSetContainers(container.GetChildren()),
					Volumes: []corev1.Volume{
						{
							Name: kube.Config.Kubernetes.Volume,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: kube.Config.Kubernetes.Volume,
								},
							},
						},
					},
				},
			},
		},
	}

	result, err := client.Create(context.TODO(), daemonset, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}

	container.State = "Creating"
	log.Info("Created daemon set ", result.GetObjectMeta().GetName())
}

// DEPLOYMENT METHODS

func (kube *Kubernetes) GetDeployment(name string) *appsv1.Deployment {
	deployment, _ := kube.ClientSet.AppsV1().Deployments(kube.Config.Kubernetes.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})
	return deployment
}

func (kube *Kubernetes) DeploymentExists(name string) bool {
	list, err := kube.ClientSet.AppsV1().Deployments(kube.Config.Kubernetes.Namespace).List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error(err)
	}

	for _, item := range list.Items {
		if item.Name == name {
			return true
		}
	}
	return false
}

func (kube *Kubernetes) GetDeploymentContainers(instances []*pipeline.Command) []corev1.Container {
	containers := make([]corev1.Container, 0)
	for _, instance := range instances {
		container := corev1.Container{
			Name:            instance.Name,
			Image:           instance.Tag,
			ImagePullPolicy: corev1.PullAlways,
			Ports:           kube.GetContainerPorts(instance),
		}
		containers = append(containers, container)
	}
	return containers
}

func (kube *Kubernetes) DestroyDeployment(pipeline string, container *pipeline.Container) {
	var name string = pipeline + "-" + container.Name
	log.Info("Deleting Deployment ", name)
	client := kube.ClientSet.AppsV1().Deployments(kube.Config.Kubernetes.Namespace)
	policy := metav1.DeletePropagationForeground
	client.Delete(context.TODO(), name, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})

	for {
		if !kube.DeploymentExists(name) {
			log.Info("Deleted deployment ", name)
			break
		}
		log.Info("Still deleting deployment ", name)
		time.Sleep(1 * time.Second)
	}

	if kube.ServiceExists(name) {
		kube.DestroyService(name)
	}
}

func (kube *Kubernetes) CreateDeployment(pipeline string, container *pipeline.Container) {
	var name string = pipeline + "-" + container.Name
	instances := container.GetChildren()

	// If the deployment exists delete and recreate it.
	if kube.DeploymentExists(name) {
		container.State = "Terminating"
		kube.DestroyDeployment(pipeline, container)
	}

	client := kube.ClientSet.AppsV1().Deployments(kube.Config.Kubernetes.Namespace)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: kube.Config.Kubernetes.Namespace,
			Labels: map[string]string{
				"app": pipeline,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &container.Scale,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: kube.GetDeploymentContainers(instances),
				},
			},
		},
	}

	result, err := client.Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	container.State = "Creating"
	log.Info("Created deployment ", result.GetObjectMeta().GetName())
}

// MISC METHODS

func (kube *Kubernetes) IsExistingResource(name string) bool {
	return kube.DeploymentExists(name) || kube.StatefulSetExists(name) || kube.DaemonSetExists(name)
}

func (kube *Kubernetes) GetVolumeMountForNamespace(namespace string) []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, 0)
	mount := corev1.VolumeMount{
		Name:      kube.Config.Kubernetes.Volume,
		MountPath: kube.Config.SequenceBaseDir,
	}
	mounts = append(mounts, mount)
	return mounts
}

func (kube *Kubernetes) CreateServiceAccount() {}

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

func (kube *Kubernetes) GetServicePorts(instances []*pipeline.Command) []corev1.ServicePort {
	ports := make([]corev1.ServicePort, 0)

	for _, instance := range instances {
		if instance.ExposePort > 0 {
			port := corev1.ServicePort{}
			log.Debug("Found ExposePort port ", instance.ExposePort)
			port.Port = int32(instance.ExposePort)
			port.TargetPort = intstr.FromInt(instance.ExposePort)
			log.Debug("Found target port ", port.TargetPort)
			port.Protocol = corev1.ProtocolTCP
			if instance.IsUDP {
				port.Protocol = corev1.ProtocolUDP
			}
			ports = append(ports, port)
		}

		links := kube.Pipeline.GetLinksTo(instance)
		for _, link := range links {
			switch (*link).(type) {
			case *pipeline.PortLink:
				if (*link).(*pipeline.PortLink).DestPort > 0 {
					port := corev1.ServicePort{}
					port.Port = int32((*link).(*pipeline.PortLink).DestPort)
					port.TargetPort = intstr.FromInt((*link).(*pipeline.PortLink).DestPort)
					port.Protocol = corev1.ProtocolTCP
					if (*link).GetType() == "udp" {
						port.Protocol = corev1.ProtocolUDP
					}

					var found bool = false
					for _, p := range ports {
						if port.TargetPort == p.TargetPort && port.Protocol == p.Protocol {
							found = true
						}
					}
					if !found {
						ports = append(ports, port)
					}
				}
			}
		}
	}

	return ports
}

func (kube *Kubernetes) CreateService(name string, instances []*pipeline.Command) string {
	ports := kube.GetServicePorts(instances)
	log.Debug("Found ", len(ports), " ports for service ", name)
	if len(ports) == 0 {
		log.Info("Not creating ", name, " no ports to bind")
		return ""
	}
	log.Info("Creating service ", name)
	client := kube.ClientSet.CoreV1().Services(kube.Config.Kubernetes.Namespace)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.ServiceSpec{
			Ports: ports,
			Type:  corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": name,
			},
		},
	}
	result, err := client.Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		// service probably already exists
		// TODO, enhance error check/destroy/recreate
		return name
	}

	log.Info("Created service ", result.GetObjectMeta().GetName())
	kube.CreateIngress(name, instances)
	return result.GetObjectMeta().GetName()
}

func (kube *Kubernetes) DestroyService(name string) {
	client := kube.ClientSet.CoreV1().Services(kube.Config.Kubernetes.Namespace)
	policy := metav1.DeletePropagationForeground
	client.Delete(context.TODO(), name, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})

	// separately destroy the associated ingress
	go kube.DestroyIngress(name)
	for {
		if !kube.ServiceExists(name) {
			log.Info("Deleted service ", name)
			break
		}
		log.Info("Still deleting service ", name)
		time.Sleep(1 * time.Second)
	}
}

func (kube *Kubernetes) ServiceExists(name string) bool {
	list, err := kube.ClientSet.CoreV1().Services(kube.Config.Kubernetes.Namespace).List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error(err)
	}

	for _, item := range list.Items {
		if item.Name == name {
			return true
		}
	}
	return false
}

// ServicePort : Address and port information for sending into NGINX
type ServicePort struct {

	// The node IP address
	Address string

	// The service node port
	Port int32
}

func (kube *Kubernetes) ServiceNodePorts(name string) *[]ServicePort {
	list, err := kube.ClientSet.CoreV1().Services(kube.Config.Kubernetes.Namespace).List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error(err)
	}

	nodeAddresses := kube.GetExternalNodeIPs()
	log.Debug(fmt.Sprintf("ServiceNodePorts: Got %d node addresses for name %s", len(nodeAddresses), name))
	servicePorts := make([]ServicePort, 0)
	for _, item := range list.Items {
		if item.Name != name {
			continue
		}

		log.Debug("Got service spec ports ", len(item.Spec.Ports))
		for _, port := range item.Spec.Ports {
			for _, addr := range nodeAddresses {
				servicePort := ServicePort{}
				servicePort.Address = addr
				servicePort.Port = port.NodePort
				log.Debug("Adding port ", servicePort.Port)
				servicePorts = append(servicePorts, servicePort)
			}
		}
	}
	log.Debug(fmt.Sprintf("Found external node service ports %+v", servicePorts))
	return &servicePorts
}

func (kube *Kubernetes) IngressRules(serviceName string, instances []*pipeline.Command) []networkv1.IngressRule {
	rules := make([]networkv1.IngressRule, 0)
	paths := make([]networkv1.HTTPIngressPath, 0)
	added := make([]int, 0)

	for _, instance := range instances {
		pathType := networkv1.PathTypePrefix
		if instance.ExposePort > 0 {
			path := networkv1.HTTPIngressPath{
				Path:     "/" + instance.Name,
				PathType: &pathType,
				Backend: networkv1.IngressBackend{
					Service: &networkv1.IngressServiceBackend{
						Name: serviceName,
						Port: networkv1.ServiceBackendPort{
							Number: int32(instance.ExposePort),
						},
					},
				},
			}
			paths = append(paths, path)
			added = append(added, instance.ExposePort)
		}

		links := kube.Pipeline.GetLinksTo(instance)
		for _, link := range links {
			switch (*link).(type) {
			case *pipeline.PortLink:
				if (*link).(*pipeline.PortLink).DestPort > 0 {
					// only add TCP to the ingress
					if (*link).GetType() != "tcp" {
						continue
					}

					var port int = (*link).(*pipeline.PortLink).DestPort
					// check it's not previously been added
					var found bool = false
					for _, p := range added {
						if port == p {
							found = true
						}
					}
					if found {
						continue
					}
					path := networkv1.HTTPIngressPath{
						Path:     "/" + instance.Name + "/" + string(port),
						PathType: &pathType,
						Backend: networkv1.IngressBackend{
							Service: &networkv1.IngressServiceBackend{
								Name: serviceName,
								Port: networkv1.ServiceBackendPort{
									Number: int32(port),
								},
							},
						},
					}
					paths = append(paths, path)
					added = append(added, port)
				}
			}
		}
	}

	// send back empty ruleset if paths is empty
	if len(paths) == 0 {
		return rules
	}

	rule := networkv1.IngressRule{
		Host: kube.Pipeline.Fqdn,
		IngressRuleValue: networkv1.IngressRuleValue{
			HTTP: &networkv1.HTTPIngressRuleValue{
				Paths: paths,
			},
		},
	}
	rules = append(rules, rule)

	if kube.Config.ExternalNginx {
		servicePorts := kube.ServiceNodePorts(serviceName)
		log.Debug("Firing Create nginx config with ", rules, servicePorts)
		container := kube.Pipeline.ContainerFromServiceName(serviceName)
		if container != nil {
			CreateNginxConfig(kube.Config, container.Name, rules, servicePorts)
		}
	}

	return rules
}

func (kube *Kubernetes) CreateIngress(ingressName string, instances []*pipeline.Command) {
	log.Info("Creating ingress ", ingressName)
	client := kube.ClientSet.NetworkingV1().Ingresses(kube.Config.Kubernetes.Namespace)
	ingress := &networkv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: ingressName,
		},
		Spec: networkv1.IngressSpec{
			Rules: kube.IngressRules(ingressName, instances),
		},
	}

	// If the Ingress exists delete and recreate it.
	if kube.IngressExists(ingressName) {
		kube.DestroyIngress(ingressName)
	}

	result, err := client.Create(context.TODO(), ingress, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	log.Info("Created Ingress ", result.GetObjectMeta().GetName())

}

func (kube *Kubernetes) DestroyIngress(name string) {
	client := kube.ClientSet.NetworkingV1beta1().Ingresses(kube.Config.Kubernetes.Namespace)
	policy := metav1.DeletePropagationForeground
	client.Delete(context.TODO(), name, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})

	for {
		if !kube.IngressExists(name) {
			log.Info("Deleted ingress ", name)
			break
		}
		log.Info("Still deleting ingress ", name)
		time.Sleep(1 * time.Second)
	}
}

func (kube *Kubernetes) IngressExists(name string) bool {
	list, err := kube.ClientSet.NetworkingV1beta1().Ingresses(kube.Config.Kubernetes.Namespace).List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error(err)
	}

	for _, item := range list.Items {
		if item.Name == name {
			return true
		}
	}
	return false
}

func (kube *Kubernetes) CreateNamespace() {
	spec := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: kube.Config.Kubernetes.Namespace,
		},
	}
	kube.ClientSet.CoreV1().Namespaces().Create(context.TODO(), spec, metav1.CreateOptions{})
}
