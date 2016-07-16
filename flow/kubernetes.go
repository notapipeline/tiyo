package flow

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/mproffitt/tiyo/config"
	"github.com/mproffitt/tiyo/pipeline"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Kubernetes struct {
	KubeConfig *rest.Config
	ClientSet  *kubernetes.Clientset
	Config     *config.Config
	Pipeline   *pipeline.Pipeline
	NameExp    *regexp.Regexp
}

func NewKubernetes(config *config.Config, pipeline *pipeline.Pipeline) *Kubernetes {
	kube := Kubernetes{
		Pipeline: pipeline,
		Config:   config,
	}

	var err error
	kube.NameExp, _ = regexp.Compile("[^A-Za-z0-9]+")

	kube.KubeConfig, err = clientcmd.BuildConfigFromFlags("", config.Kubernetes.ConfigFile)
	if err != nil {
		panic(err)
	}

	kube.ClientSet, err = kubernetes.NewForConfig(kube.KubeConfig)
	if err != nil {
		panic(err)
	}

	return &kube
}

func (kube *Kubernetes) GetPod(name string) *corev1.Pod {
	pod, _ := kube.ClientSet.CoreV1().Pods(kube.Config.Kubernetes.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})
	return pod
}

func (kube *Kubernetes) GetStatefulSet(name string) *appsv1.StatefulSet {
	state, _ := kube.ClientSet.AppsV1().StatefulSets(kube.Config.Kubernetes.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})
	return state
}

func (kube *Kubernetes) GetDeployment(name string) *appsv1.Deployment {
	deployment, _ := kube.ClientSet.AppsV1().Deployments(kube.Config.Kubernetes.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})
	return deployment
}

func (kube *Kubernetes) PodExists(name string) bool {
	return kube.GetPod(name) != nil
}

func (kube *Kubernetes) StatefulSetExists(name string) bool {
	return kube.GetStatefulSet(name) != nil
}

func (kube *Kubernetes) DeploymentExists(name string) bool {
	return kube.GetDeployment(name) != nil
}

func (kube *Kubernetes) GetMaxReplicas(instances []*pipeline.Command) *int32 {
	// set at 30 but should be ((nodes/max_containers_in_pipeline) - persistant_containers) / (CPU|RAM|CAPACITY)
	// instance.Scale must always be less than this number - if not, it gets truncated.
	var scale int32 = int32(30)
	for _, instance := range instances {
		if int32(instance.Scale) > scale {
			scale = int32(instance.Scale)
		}
	}
	return &scale
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

func (kube *Kubernetes) GetDeploymentContainers(instances []*pipeline.Command) []corev1.Container {
	containers := make([]corev1.Container, 0)
	for _, instance := range instances {
		container := corev1.Container{
			Name:  instance.Name,
			Image: instance.Image,
			Ports: kube.GetContainerPorts(instance),
		}
		containers = append(containers, container)
	}
	return containers
}

func (kube *Kubernetes) GetVolumeMountForNamespace(namespace string) []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, 0)
	// TODO: Get volume mounts and attach to list
	return mounts
}

func (kube *Kubernetes) GetStatefulSetContainers(instances []*pipeline.Command) []corev1.Container {
	containers := make([]corev1.Container, 0)
	for _, instance := range instances {
		container := corev1.Container{
			Name:         instance.Name,
			Image:        instance.Image,
			Ports:        kube.GetContainerPorts(instance),
			VolumeMounts: kube.GetVolumeMountForNamespace(kube.Config.Kubernetes.Namespace),
		}
		containers = append(containers, container)
	}
	return containers
}

func (kube *Kubernetes) CreateDeployment(pipelineName string, groupName string, instances []*pipeline.Command) {
	name := strings.Trim(kube.NameExp.ReplaceAllString(groupName, "-"), "-")
	client := kube.ClientSet.AppsV1().Deployments(kube.Config.Kubernetes.Namespace)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", pipelineName, name),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: kube.GetMaxReplicas(instances),
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
	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())
}

func (kube *Kubernetes) CreateStatefulSet(pipelineName string, groupName string, instances []*pipeline.Command) {
	name := strings.Trim(kube.NameExp.ReplaceAllString(groupName, "-"), "-")
	client := kube.ClientSet.AppsV1().StatefulSets(kube.Config.Kubernetes.Namespace)

	statefuleset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", pipelineName, name),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: kube.GetMaxReplicas(instances),
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
	result, err := client.Create(context.TODO(), statefuleset, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())
}

func (kube *Kubernetes) CreateDaemonSet(pipelineName string, groupName string, instances []*pipeline.Command) {
	name := strings.Trim(kube.NameExp.ReplaceAllString(groupName, "-"), "-")
	client := kube.ClientSet.AppsV1().DaemonSets(kube.Config.Kubernetes.Namespace)

	daemonset := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", pipelineName, name),
			Namespace: kube.Config.Kubernetes.Namespace,
			Labels: map[string]string{
				"k8s-app": pipelineName,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": name,
					},
				},
				Spec: corev1.PodSpec{
					Tolerations: []corev1.Toleration{
						{
							Key:    "node-role.kubernetes.io/master",
							Effect: "NoSchedule",
						},
					},
					Containers: kube.GetDeploymentContainers(instances),
				},
			},
		},
	}
	result, err := client.Create(context.TODO(), daemonset, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())
}
