// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (kube *Kubernetes) CreateNamespace(name string) {
	spec := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	kube.ClientSet.CoreV1().Namespaces().Create(context.TODO(), spec, metav1.CreateOptions{})
}

func (kube *Kubernetes) GetNamespace(name string) *corev1.Namespace {
	namespace, err := kube.ClientSet.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to retrieve namespace %s - error was %v", name, err)
	}
	return namespace
}

func (kube *Kubernetes) DestroyNamespace(name string) {
	if err := kube.ClientSet.CoreV1().Namespaces().Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		log.Errorf("Failed to delete namespace %s - error was %v", name, err)
	}
}

func (kube *Kubernetes) ListNamespaces() []string {
	var namespaces []string = make([]string, 0)
	n, err := kube.ClientSet.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error("Cannot retrieve namespace list ", err)
	}
	for _, item := range n.Items {
		namespaces = append(namespaces, item.Name)
	}
	return namespaces
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
