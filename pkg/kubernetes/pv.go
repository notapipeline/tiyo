package kubernetes

import (
	"context"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (kube *Kubernetes) GetPv(name string) *corev1.PersistentVolume {
	volume, _ := kube.ClientSet.CoreV1().PersistentVolumes().Get(
		context.TODO(), name, metav1.GetOptions{})
	return volume
}

// PodExists : Check that a pod exists
func (kube *Kubernetes) PvExists(name string) bool {
	list, err := kube.ClientSet.CoreV1().PersistentVolumes().List(
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
