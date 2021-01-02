package flow

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/choclab-net/tiyo/config"
	"github.com/choclab-net/tiyo/pipeline"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	log.Info("Initialising Kubernetes engine")
	kube := Kubernetes{
		Pipeline: pipeline,
		Config:   config,
	}

	var err error
	kube.NameExp, _ = regexp.Compile("[^A-Za-z0-9]+")

	log.Info("Loading config file from ", config.Kubernetes.ConfigFile)
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
	statefulset, _ := kube.ClientSet.AppsV1().StatefulSets(kube.Config.Kubernetes.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})
	return statefulset
}

func (kube *Kubernetes) GetDaemonSet(name string) *appsv1.DaemonSet {
	daemonset, _ := kube.ClientSet.AppsV1().DaemonSets(kube.Config.Kubernetes.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})
	return daemonset
}

func (kube *Kubernetes) GetDeployment(name string) *appsv1.Deployment {
	deployment, _ := kube.ClientSet.AppsV1().Deployments(kube.Config.Kubernetes.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})
	return deployment
}

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

func (kube *Kubernetes) IsExistingResource(name string) bool {
	return kube.DeploymentExists(name) || kube.StatefulSetExists(name)
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

func (kube *Kubernetes) GetServicePorts(instances []*pipeline.Command) []corev1.ServicePort {
	ports := make([]corev1.ServicePort, 0)

	for _, instance := range instances {
		links := kube.Pipeline.GetLinksTo(instance)
		for _, link := range links {
			switch (*link).(type) {
			case *pipeline.PortLink:
				port := corev1.ServicePort{}
				port.TargetPort = intstr.FromInt((*link).(*pipeline.PortLink).DestPort)
				port.Name = (*link).GetType() + "-" + string((*link).(*pipeline.PortLink).DestPort)
				port.Protocol = corev1.ProtocolTCP
				if (*link).GetType() == "udp" {
					port.Protocol = corev1.ProtocolUDP
				}
			}
		}
	}

	return ports
}

func (kube *Kubernetes) GetDeploymentContainers(instances []*pipeline.Command) []corev1.Container {
	containers := make([]corev1.Container, 0)
	for _, instance := range instances {
		container := corev1.Container{
			Name:            instance.Name,
			Image:           instance.Image,
			ImagePullPolicy: corev1.PullAlways,
			Ports:           kube.GetContainerPorts(instance),
		}
		containers = append(containers, container)
	}
	return containers
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

func (kube *Kubernetes) GetStatefulSetContainers(instances []*pipeline.Command) []corev1.Container {
	containers := make([]corev1.Container, 0)
	for _, instance := range instances {
		container := corev1.Container{
			Name:            instance.Name,
			Image:           instance.Image,
			ImagePullPolicy: corev1.PullAlways,
			Ports:           kube.GetContainerPorts(instance),
			VolumeMounts:    kube.GetVolumeMountForNamespace(kube.Config.Kubernetes.Namespace),
		}
		containers = append(containers, container)
	}
	return containers
}

func (kube *Kubernetes) CreateServiceAccount() {}

func (kube *Kubernetes) CreateService(serviceName string, instances []*pipeline.Command) string {
	client := kube.ClientSet.CoreV1().Services(kube.Config.Kubernetes.Namespace)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: corev1.ServiceSpec{
			Ports: kube.GetServicePorts(instances),
		},
	}

	result, err := client.Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created service %q.\n", result.GetObjectMeta().GetName())
	return result.GetObjectMeta().GetName()
}

func (kube *Kubernetes) CreateNamespace() {
	spec := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: kube.Config.Kubernetes.Namespace,
		},
	}
	kube.ClientSet.CoreV1().Namespaces().Create(context.TODO(), spec, metav1.CreateOptions{})
}

func (kube *Kubernetes) DeleteDeployment(name string) {
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
		time.Sleep(1 * time.Second)
	}
}

func (kube *Kubernetes) CreateDeployment(pipeline string, name string, instances []*pipeline.Command) {
	var depName string
	pipeline, name, depName = kube.Sanitize(pipeline, name)

	client := kube.ClientSet.AppsV1().Deployments(kube.Config.Kubernetes.Namespace)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: depName,
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

func (kube *Kubernetes) CreateStatefulSet(pipeline string, name string, instances []*pipeline.Command) {
	var stateName string
	pipeline, name, stateName = kube.Sanitize(pipeline, name)
	client := kube.ClientSet.AppsV1().StatefulSets(kube.Config.Kubernetes.Namespace)

	log.Info("Creating statefule set ", pipeline, " ", name)
	statefuleset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stateName,
			Namespace: kube.Config.Kubernetes.Namespace,
			Labels: map[string]string{
				"k8s-app": stateName,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: kube.GetMaxReplicas(instances),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			ServiceName: kube.CreateService(stateName, instances),
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

	// If the stateful set exists, delete and recreate it.
	if kube.StatefulSetExists(stateName) {
		kube.DeleteStatefulSet(name)
	}

	// otherwise create
	result, err := client.Create(context.TODO(), statefuleset, metav1.CreateOptions{})
	if err != nil {
		log.Panic(err)
	}
	log.Info("Created StatefulSet ", result.GetObjectMeta().GetName())
}

func (kube *Kubernetes) DeleteStatefulSet(name string) {
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
		time.Sleep(1 * time.Second)
	}
}

func (kube *Kubernetes) DeleteDaemonSet(name string) {
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
		time.Sleep(1 * time.Second)
	}
}

func (kube *Kubernetes) CreateDaemonSet(pipeline string, name string, instances []*pipeline.Command) {
	var daemonName string
	pipeline, name, daemonName = kube.Sanitize(pipeline, name)
	client := kube.ClientSet.AppsV1().DaemonSets(kube.Config.Kubernetes.Namespace)

	daemonset := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonName,
			Namespace: kube.Config.Kubernetes.Namespace,
			Labels: map[string]string{
				"k8s-app": daemonName,
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
	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())
}

func (kube *Kubernetes) Sanitize(pipeline string, name string) (string, string, string) {
	pipeline = strings.Trim(kube.NameExp.ReplaceAllString(pipeline, "-"), "-")
	name = strings.Trim(kube.NameExp.ReplaceAllString(name, "-"), "-")
	return pipeline, name, strings.ToLower(fmt.Sprintf("%s-%s", pipeline, name))
}
