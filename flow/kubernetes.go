package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/choclab-net/tiyo/config"
	"github.com/choclab-net/tiyo/pipeline"
	"github.com/choclab-net/tiyo/server"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type ContainerStatus struct {
	Id     string `json:"id"`
	State  string `json:"state"`
	Reason string `json:"reason"`
}

type PodsStatus struct {
	State      string                     `json:"state"`
	Containers map[string]ContainerStatus `json:"containers"`
}

type Kubernetes struct {
	KubeConfig *rest.Config
	ClientSet  *kubernetes.Clientset
	Config     *config.Config
	Pipeline   *pipeline.Pipeline
	NameExp    *regexp.Regexp
}

func NewKubernetes(config *config.Config, pipeline *pipeline.Pipeline) (*Kubernetes, error) {
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
		return nil, err
	}

	kube.ClientSet, err = kubernetes.NewForConfig(kube.KubeConfig)
	if err != nil {
		return nil, err
	}

	return &kube, nil
}

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

func (kube *Kubernetes) GetPod(name string) *corev1.Pod {
	pod, _ := kube.ClientSet.CoreV1().Pods(kube.Config.Kubernetes.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})
	return pod
}

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
		podname := pod.Name
		podStatus := PodsStatus{}
		podStatus.State = string(pod.Status.Phase)

		cstatus := make(map[string]ContainerStatus)
		for _, container := range pod.Status.ContainerStatuses {
			image := container.Image
			var (
				state  string = "Waiting"
				reason string = ""
			)
			if container.State.Running != nil {
				state = kube.getStateFromDb(podname, image)
				// if any container is busy, pod state is executing
				if state == "Busy" {
					podStatus.State = "Executing"
				}
			} else if container.State.Terminated != nil {
				state = "Terminated"
				reason = container.State.Terminated.Reason
			}
			if _, ok := cstatus[image]; !ok {
				var c *pipeline.Command = kube.Pipeline.CommandFromImageName(image)
				var id string = ""
				if c != nil {
					id = c.Id
				}
				cstatus[image] = ContainerStatus{
					Id:     id,
					State:  state,
					Reason: reason,
				}
			}
		}
		podStatus.Containers = cstatus
		statuses[podname] = podStatus
	}
	return statuses, nil
}

// Gets any additional reported state from the database
//
// On any error, this will return "running" as this is the default
// state the pod is in.
//
// If no error is detected, the state will be one of "ready"|"busy"
func (kube *Kubernetes) getStateFromDb(podname string, image string) string {
	var slice []string = strings.Split(image, "/")
	var (
		state   string = "Running"
		name    string = slice[len(slice)-1] + ":" + podname
		address        = kube.Config.AssembleServer() + "/api/v1/bucket/pods/" + kube.Pipeline.BucketName + "/" + name
	)

	request, err := http.NewRequest("GET", address, nil)
	if err != nil {
		log.Error(err)
		return state
	}
	request.Header.Set("Accept", "application/vnd.github.v3+json")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		log.Error(err)
		return state
	}
	defer response.Body.Close()
	if response.StatusCode == 200 {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Error(err)
			return state
		}
		contents := server.Result{}
		err = json.Unmarshal(body, &contents)
		if err != nil {
			log.Error(err)
			return state
		}
		state = string(contents.Message.(string))
	}

	return state
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

// STATEFUL SET METHODS

func (kube *Kubernetes) GetStatefulSet(name string) *appsv1.StatefulSet {
	statefulset, _ := kube.ClientSet.AppsV1().StatefulSets(kube.Config.Kubernetes.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})
	return statefulset
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

func (kube *Kubernetes) GetStatefulSetContainers(instances []*pipeline.Command) []corev1.Container {
	containers := make([]corev1.Container, 0)
	for _, instance := range instances {
		container := corev1.Container{
			Name:            instance.Name,
			Image:           instance.Tag,
			ImagePullPolicy: corev1.PullAlways,
			Ports:           kube.GetContainerPorts(instance),
			VolumeMounts:    kube.GetVolumeMountForNamespace(kube.Config.Kubernetes.Namespace),
		}
		containers = append(containers, container)
	}
	return containers
}

func (kube *Kubernetes) DestroyStatefulSet(pipeline string, container *pipeline.Container) {
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
		log.Info("Still deleting statefulset", name)
		time.Sleep(1 * time.Second)
	}

	if kube.ServiceExists(name) {
		kube.DestroyService(name)
	}
}

func (kube *Kubernetes) CreateStatefulSet(pipeline string, container *pipeline.Container) {
	var (
		name             = container.Name
		stateName string = pipeline + "-" + container.Name
	)
	client := kube.ClientSet.AppsV1().StatefulSets(kube.Config.Kubernetes.Namespace)
	instances := container.GetChildren()

	log.Info("Creating stateful set ", stateName)
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stateName,
			Namespace: kube.Config.Kubernetes.Namespace,
			Labels: map[string]string{
				"k8s-app": stateName,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &container.Scale,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			ServiceName: kube.CreateService(stateName, name, instances),
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
		container.State = "Terminating"
		kube.DestroyStatefulSet(pipeline, container)
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

// DAEMON SET METHODS

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

func (kube *Kubernetes) DestroyDaemonSet(pipeline string, container *pipeline.Container) {
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

func (kube *Kubernetes) CreateDaemonSet(pipeline string, container *pipeline.Container) {
	var (
		name       string = container.Name
		daemonName string = pipeline + "-" + container.Name
	)
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
					Containers: kube.GetStatefulSetContainers(container.GetChildren()),
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

	// If the daemonset exists, delete and recreate it.
	if kube.DaemonSetExists(daemonName) {
		container.State = "Terminating"
		kube.DestroyDaemonSet(pipeline, container)
	}

	result, err := client.Create(context.TODO(), daemonset, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}

	container.State = "Creating"
	log.Info("Created daemon set ", result.GetObjectMeta().GetName())
}

// DEPLOYMENT METHODS

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

func (kube *Kubernetes) DestroyDeployment(pipeline string, container *pipeline.Container) {
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

func (kube *Kubernetes) CreateDeployment(pipeline string, container *pipeline.Container) {
	var name string = container.Name
	var depName string = pipeline + "-" + container.Name
	instances := container.GetChildren()

	client := kube.ClientSet.AppsV1().Deployments(kube.Config.Kubernetes.Namespace)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: depName,
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

	// If the deployment exists delete and recreate it.
	if kube.DeploymentExists(depName) {
		container.State = "Terminating"
		kube.DestroyDeployment(pipeline, container)
	}

	result, err := client.Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	container.State = "Creating"
	log.Info("Created deployment ", result.GetObjectMeta().GetName())
}

// MISC METHODS

func (kube *Kubernetes) IsExistingResource(name string) bool {
	return kube.DeploymentExists(name) || kube.StatefulSetExists(name) || kube.DaemonSetExists(name)
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

func (kube *Kubernetes) CreateServiceAccount() {}

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
		if instance.ExposePort > 0 {
			port := corev1.ServicePort{}
			log.Debug("Found ExposePort port ", instance.ExposePort)
			port.Port = int32(instance.ExposePort)
			port.TargetPort = intstr.FromInt(instance.ExposePort)
			log.Debug("Found target port ", port.TargetPort)
			port.Protocol = corev1.ProtocolTCP
			if instance.IsUdp {
				port.Protocol = corev1.ProtocolUDP
			}
			ports = append(ports, port)
		}

		links := kube.Pipeline.GetLinksTo(instance)
		for _, link := range links {
			switch (*link).(type) {
			case *pipeline.PortLink:
				if (*link).(*pipeline.PortLink).DestPort > 0 {
					port := corev1.ServicePort{}
					port.Port = int32((*link).(*pipeline.PortLink).DestPort)
					port.TargetPort = intstr.FromInt((*link).(*pipeline.PortLink).DestPort)
					port.Protocol = corev1.ProtocolTCP
					if (*link).GetType() == "udp" {
						port.Protocol = corev1.ProtocolUDP
					}

					var found bool = false
					for _, p := range ports {
						if port.TargetPort == p.TargetPort && port.Protocol == p.Protocol {
							found = true
						}
					}
					if !found {
						ports = append(ports, port)
					}
				}
			}
		}
	}

	return ports
}

func (kube *Kubernetes) CreateService(serviceName string, selector string, instances []*pipeline.Command) string {
	ports := kube.GetServicePorts(instances)
	log.Debug("Found ", len(ports), " ports for service ", serviceName)
	if len(ports) == 0 {
		log.Info("Not creating ", serviceName, " no ports to bind")
		return ""
	}
	log.Info("Creating service ", serviceName)
	client := kube.ClientSet.CoreV1().Services(kube.Config.Kubernetes.Namespace)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: corev1.ServiceSpec{
			Ports: ports,
			Type:  corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": selector,
			},
		},
	}
	result, err := client.Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		// service probably already exists
		// TODO, enhance error check/destroy/recreate
		return serviceName
	}

	log.Info("Created service ", result.GetObjectMeta().GetName())
	kube.CreateIngress(serviceName, instances)
	return result.GetObjectMeta().GetName()
}

func (kube *Kubernetes) DestroyService(name string) {
	client := kube.ClientSet.CoreV1().Services(kube.Config.Kubernetes.Namespace)
	policy := metav1.DeletePropagationForeground
	client.Delete(context.TODO(), name, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})

	// separately destroy the associated ingress
	go kube.DestroyIngress(name)
	for {
		if !kube.ServiceExists(name) {
			log.Info("Deleted service ", name)
			break
		}
		log.Info("Still deleting service ", name)
		time.Sleep(1 * time.Second)
	}
}

func (kube *Kubernetes) ServiceExists(name string) bool {
	list, err := kube.ClientSet.CoreV1().Services(kube.Config.Kubernetes.Namespace).List(
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

type ServicePort struct {
	Address string
	Port    int32
}

func (kube *Kubernetes) ServiceNodePorts(name string) *[]ServicePort {
	list, err := kube.ClientSet.CoreV1().Services(kube.Config.Kubernetes.Namespace).List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error(err)
	}

	nodeAddresses := kube.GetExternalNodeIPs()
	log.Debug(fmt.Sprintf("ServiceNodePorts: Got %d node addresses for name %s", len(nodeAddresses), name))
	servicePorts := make([]ServicePort, 0)
	for _, item := range list.Items {
		if item.Name != name {
			continue
		}

		log.Debug("Got service spec ports ", len(item.Spec.Ports))
		for _, port := range item.Spec.Ports {
			for _, addr := range nodeAddresses {
				servicePort := ServicePort{}
				servicePort.Address = addr
				servicePort.Port = port.NodePort
				log.Debug("Adding port ", servicePort.Port)
				servicePorts = append(servicePorts, servicePort)
			}
		}
	}
	log.Debug(fmt.Sprintf("Found external node service ports %+v", servicePorts))
	return &servicePorts
}

func (kube *Kubernetes) IngressRules(serviceName string, instances []*pipeline.Command) []networkv1.IngressRule {
	rules := make([]networkv1.IngressRule, 0)
	paths := make([]networkv1.HTTPIngressPath, 0)
	added := make([]int, 0)

	for _, instance := range instances {
		pathType := networkv1.PathTypePrefix
		if instance.ExposePort > 0 {
			path := networkv1.HTTPIngressPath{
				Path:     "/" + instance.Name,
				PathType: &pathType,
				Backend: networkv1.IngressBackend{
					Service: &networkv1.IngressServiceBackend{
						Name: serviceName,
						Port: networkv1.ServiceBackendPort{
							Number: int32(instance.ExposePort),
						},
					},
				},
			}
			paths = append(paths, path)
			added = append(added, instance.ExposePort)
		}

		links := kube.Pipeline.GetLinksTo(instance)
		for _, link := range links {
			switch (*link).(type) {
			case *pipeline.PortLink:
				if (*link).(*pipeline.PortLink).DestPort > 0 {
					// only add TCP to the ingress
					if (*link).GetType() != "tcp" {
						continue
					}

					var port int = (*link).(*pipeline.PortLink).DestPort
					// check it's not previously been added
					var found bool = false
					for _, p := range added {
						if port == p {
							found = true
						}
					}
					if found {
						continue
					}
					path := networkv1.HTTPIngressPath{
						Path:     "/" + instance.Name + "/" + string(port),
						PathType: &pathType,
						Backend: networkv1.IngressBackend{
							Service: &networkv1.IngressServiceBackend{
								Name: serviceName,
								Port: networkv1.ServiceBackendPort{
									Number: int32(port),
								},
							},
						},
					}
					paths = append(paths, path)
					added = append(added, port)
				}
			}
		}
	}

	// send back empty ruleset if paths is empty
	if len(paths) == 0 {
		return rules
	}

	rule := networkv1.IngressRule{
		Host: kube.Pipeline.Fqdn,
		IngressRuleValue: networkv1.IngressRuleValue{
			HTTP: &networkv1.HTTPIngressRuleValue{
				Paths: paths,
			},
		},
	}
	rules = append(rules, rule)

	if kube.Config.ExternalNginx {
		servicePorts := kube.ServiceNodePorts(serviceName)
		log.Debug("Firing Create nginx config with ", rules, servicePorts)
		CreateNginxConfig(kube.Config, serviceName, rules, servicePorts)
	}

	return rules
}

func (kube *Kubernetes) CreateIngress(ingressName string, instances []*pipeline.Command) {
	log.Info("Creating ingress ", ingressName)
	client := kube.ClientSet.NetworkingV1().Ingresses(kube.Config.Kubernetes.Namespace)
	ingress := &networkv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: ingressName,
		},
		Spec: networkv1.IngressSpec{
			Rules: kube.IngressRules(ingressName, instances),
		},
	}

	// If the Ingress exists delete and recreate it.
	if kube.IngressExists(ingressName) {
		kube.DestroyIngress(ingressName)
	}

	result, err := client.Create(context.TODO(), ingress, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	log.Info("Created Ingress ", result.GetObjectMeta().GetName())

}

func (kube *Kubernetes) DestroyIngress(name string) {
	client := kube.ClientSet.NetworkingV1beta1().Ingresses(kube.Config.Kubernetes.Namespace)
	policy := metav1.DeletePropagationForeground
	client.Delete(context.TODO(), name, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})

	for {
		if !kube.IngressExists(name) {
			log.Info("Deleted ingress ", name)
			break
		}
		log.Info("Still deleting ingress ", name)
		time.Sleep(1 * time.Second)
	}
}

func (kube *Kubernetes) IngressExists(name string) bool {
	list, err := kube.ClientSet.NetworkingV1beta1().Ingresses(kube.Config.Kubernetes.Namespace).List(
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

func (kube *Kubernetes) CreateNamespace() {
	spec := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: kube.Config.Kubernetes.Namespace,
		},
	}
	kube.ClientSet.CoreV1().Namespaces().Create(context.TODO(), spec, metav1.CreateOptions{})
}
