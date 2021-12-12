package flow

import (
	"context"
	"fmt"
	"time"

	"github.com/notapipeline/tiyo/pkg/pipeline"
	log "github.com/sirupsen/logrus"
	networkv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (kube *Kubernetes) IngressRules(serviceName string, instances []*pipeline.Command) []networkv1.IngressRule {
	rules := make([]networkv1.IngressRule, 0)
	paths := make([]networkv1.HTTPIngressPath, 0)
	added := make([]int, 0)

	for _, instance := range instances {
		pathType := networkv1.PathTypePrefix
		if instance.ExposePort > 0 {
			path := networkv1.HTTPIngressPath{
				Path:     "/",
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
						Path:     "/",
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
		Host: fmt.Sprintf("%s.%s", serviceName, kube.Config.DNSName),
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
		container := kube.Pipeline.ContainerFromServiceName(serviceName)
		if container != nil {
			CreateNginxConfig(kube.Config, serviceName, rules, servicePorts)
		}
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
	client := kube.ClientSet.NetworkingV1().Ingresses(kube.Config.Kubernetes.Namespace)
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
	list, err := kube.ClientSet.NetworkingV1().Ingresses(kube.Config.Kubernetes.Namespace).List(
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
