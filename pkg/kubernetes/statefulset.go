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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
func (kube *Kubernetes) DestroyStatefulSet(pipeline string, container *pipeline.Controller) {
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
func (kube *Kubernetes) CreateStatefulSet(pipeline string, container *pipeline.Controller) {
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
