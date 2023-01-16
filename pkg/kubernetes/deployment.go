// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"time"

	"github.com/notapipeline/tiyo/pkg/pipeline"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

func (kube *Kubernetes) DestroyDeployment(pipeline string, container *pipeline.Controller) {
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

func (kube *Kubernetes) CreateDeployment(pipeline string, container *pipeline.Controller) {
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
	kube.CreateService(name, instances)
}
