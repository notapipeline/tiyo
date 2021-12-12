package flow

import (
	"context"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
