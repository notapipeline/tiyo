// Copyright 2022 The Tiyo authors
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

func (kube *Kubernetes) DestroyDaemonSet(pipeline string, container *pipeline.Controller) {
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

func (kube *Kubernetes) CreateDaemonSet(pipeline string, container *pipeline.Controller) {
	var name string = pipeline + "-" + container.Name
	client := kube.ClientSet.AppsV1().DaemonSets(kube.Config.Kubernetes.Namespace)

	instances := container.GetChildren()

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

	result, err := client.Create(context.TODO(), daemonset, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}

	container.State = "Creating"
	log.Info("Created daemon set ", result.GetObjectMeta().GetName())
	kube.CreateService(name, instances)
}
