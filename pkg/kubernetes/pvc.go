package kubernetes

import (
	"context"

	"github.com/notapipeline/tiyo/pkg/pipeline"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (kube *Kubernetes) GetPvc(name string) *corev1.PersistentVolumeClaim {
	claim, _ := kube.ClientSet.CoreV1().PersistentVolumeClaims(kube.Config.Kubernetes.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})
	return claim
}

// PodExists : Check that a pod exists
func (kube *Kubernetes) PvcExists(name string) bool {
	list, err := kube.ClientSet.CoreV1().PersistentVolumeClaims(kube.Config.Kubernetes.Namespace).List(
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

func (kube *Kubernetes) CreatePvc(pipeline string, container *pipeline.Controller) {
	var name string = pipeline + "-" + container.Name
	client := kube.ClientSet.CoreV1().PersistentVolumeClaims(kube.Config.Kubernetes.Namespace)
	//instance := container.GetChildren()

	// If the stateful set exists, delete it so it can be re-created.
	if kube.PvcExists(name) {
		container.State = "Terminating"
		kube.DestroyStatefulSet(pipeline, container)
	}

	claim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "",
			Namespace: "",
			Labels: map[string]string{
				"app": pipeline,
			},
			Annotations: map[string]string{},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			/*AccessModes:      instance.AccessMode,
			VolumeName:       instance.VolumeName,
			StorageClassName: instance.StorageClassName,*/
		},
	}
	// otherwise create
	result, err := client.Create(context.TODO(), claim, metav1.CreateOptions{})
	if err != nil {
		log.Panic(err)
	}
	log.Info("Created persistant volume claim ", result.GetObjectMeta().GetName())
}
