// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"fmt"

	"github.com/notapipeline/tiyo/pkg/pipeline"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
				var c *pipeline.Command = kube.Pipeline.CommandFromControllerName(name, image)
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
