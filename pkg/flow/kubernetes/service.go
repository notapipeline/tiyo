// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"fmt"
	"time"

	"github.com/notapipeline/tiyo/pkg/flow/nginx"
	"github.com/notapipeline/tiyo/pkg/pipeline"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (kube *Kubernetes) CreateServiceAccount() {}

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
			port.Name = fmt.Sprintf("tcp-%d", port.Port)
			if instance.IsUDP {
				port.Protocol = corev1.ProtocolUDP
				port.Name = fmt.Sprintf("udp-%d", port.Port)
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
					port.Name = fmt.Sprintf("%s-%d", (*link).GetType(), port.Port)
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

func (kube *Kubernetes) CreateService(name string, instances []*pipeline.Command) string {
	ports := kube.GetServicePorts(instances)
	log.Debug("Found ", len(ports), " ports for service ", name)
	if len(ports) == 0 {
		log.Infof("Not creating %s service. No ports to bind", name)
		return ""
	}
	log.Info("Creating service ", name)
	client := kube.ClientSet.CoreV1().Services(kube.Config.Kubernetes.Namespace)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.ServiceSpec{
			Ports: ports,
			Type:  corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": name,
			},
		},
	}
	result, err := client.Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("Failed to create service : %+v", err)
		// service probably already exists
		// TODO, enhance error check/destroy/recreate
		return name
	}

	log.Info("Created service ", result.GetObjectMeta().GetName())
	kube.CreateIngress(name, instances)
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

// ServicePort : Address and port information for sending into NGINX
func (kube *Kubernetes) ServiceNodePorts(name string) *[]nginx.ServicePort {
	list, err := kube.ClientSet.CoreV1().Services(kube.Config.Kubernetes.Namespace).List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error(err)
	}

	nodeAddresses := kube.GetExternalNodeIPs()
	log.Debug(fmt.Sprintf("ServiceNodePorts: Got %d node addresses for name %s", len(nodeAddresses), name))
	servicePorts := make([]nginx.ServicePort, 0)
	for _, item := range list.Items {
		if item.Name != name {
			continue
		}

		log.Debug("Got service spec ports ", len(item.Spec.Ports))
		for _, port := range item.Spec.Ports {
			for _, addr := range nodeAddresses {
				servicePort := nginx.ServicePort{}
				servicePort.Address = addr
				servicePort.Port = port.NodePort
				servicePort.Protocol = port.Protocol
				servicePort.HttpPort = kube.isHttpPort(port.TargetPort.IntVal)
				log.Debug("Adding port ", servicePort.Port)
				servicePorts = append(servicePorts, servicePort)
			}
		}
	}
	log.Debug(fmt.Sprintf("Found external node service ports %+v", servicePorts))
	return &servicePorts
}

func (kube *Kubernetes) isHttpPort(port int32) bool {
	return !(port == 443 || port == 8443 || port == 9443)
}
