// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package nginx

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/coreos/go-systemd/dbus"
	"github.com/notapipeline/tiyo/pkg/config"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
)

type ServicePort struct {

	// The node IP address
	Address string

	// The service node port
	Port int32

	// IsHttp port
	HttpPort bool

	Protocol corev1.Protocol
}

// Manages NGINX configurations when the `externalNginx` config flag is true

// Upstream : structure of an Nginx Upstream server
type Upstream struct {

	// The name to assign to the upstream - should be a valid dns identifier
	Name string

	// A list of options to assign to the nginx upstream
	Options []string

	// A list of addresses the upstream will resolve against
	Addresses []string
}

// NginxReturn : Handle an NGINX return statement
type NginxReturn struct {

	// The code to return to the client (Normally 301/302)
	Code int

	// The address to redirect against
	Address string
}

// Location : An NGINX Location to point at an upstream
type Location struct {

	// The Location path starting from /
	Path string

	// The name of an upstream to proxy against
	Upstream string

	// Skip verification check on the backend
	SkipVerify bool

	// Use HTTPS for upstream connection
	SecureUpstream bool
}

// Listener : An Nginx server endpoint to listen for requests on
type Listener struct {

	// The NGINX server port to listen on (Usually 80 or 443)
	Listen int

	// The hostname the server is listening on
	Hostname string

	// The top level domain
	Domain string

	// The Protocol to use (http|https)
	Protocol string

	// A list of location items and relevant upstream endpoints
	Locations []*Location

	// Any return value specified for this listener
	Return *NginxReturn
}

// Nginx : The structure of an NGINX server endpoint configuration
type Nginx struct {

	// The Plain HTTP upstream to build
	UpstreamPlain *Upstream

	// The secure HTTPS upstream to build
	UpstreamSecure *Upstream

	// A slice of listener objects to listen against - usually one describing 80 and one for 443
	Listeners []*Listener

	// Flow configuration for tuning
	Config *config.Config
}

// NginxServer : This structure is for handling restarts of the nginx systemd service
type NginxServer struct {

	// We lock the service to prevent actions interfering with one another
	// only one action at a time
	sync.Mutex

	// A DBUS connection to SystemD - requires root privileges
	Systemd *dbus.Conn
}

// nginxServer : There is one and only one server here.
var nginxServer *NginxServer

// CreateNginxConfig : Create an NGINX configuration file
//
// This method takes a list of Kubernetes ingress rules and associated service ports then
// attempts to build an nginx configuration file for your service.
func CreateNginxConfig(config *config.Config, name string, rules []networkv1.IngressRule, upstream *[]ServicePort) {
	log.Info("Creating NGINX config for ", name)
	var directory string = "/etc/nginx/sites.d/"

	for _, rule := range rules {
		upsPlain := Upstream{}
		upsSecure := Upstream{}
		upsPlain.Name = name
		upsSecure.Name = fmt.Sprintf("%ssecure", name)
		upsPlain.Options = []string{"least_conn;"}
		upsSecure.Options = []string{"least_conn;"}
		addressesPlain := make([]string, 0)
		addressesSecure := make([]string, 0)

		nginx := Nginx{
			UpstreamPlain:  &upsPlain,
			UpstreamSecure: &upsSecure,
		}
		nginx.Listeners = make([]*Listener, 0)

		for _, addr := range *upstream {
			if addr.HttpPort {
				log.Debug(fmt.Sprintf("Found plain http service port %+v", addr))
				addressesPlain = append(addressesPlain, fmt.Sprintf("%s:%d", addr.Address, addr.Port))
			} else {
				log.Debug(fmt.Sprintf("Found secure https service port %+v", addr))
				addressesSecure = append(addressesSecure, fmt.Sprintf("%s:%d", addr.Address, addr.Port))
			}
		}

		upsPlain.Addresses = addressesPlain
		upsSecure.Addresses = addressesSecure

		var hostname = name + "." + config.DNSName

		CreateSSLCertificates(hostname)

		// basic first round listener
		http := Listener{
			Listen:   80,
			Hostname: hostname,
			Domain:   config.DNSName,
			Protocol: "http",
			Return: &NginxReturn{
				Code:    301,
				Address: "https://$server_name$request_uri",
			},
		}
		nginx.Listeners = append(nginx.Listeners, &http)

		// basic first round listener
		https := Listener{
			Listen:   443,
			Hostname: hostname,
			Domain:   config.DNSName,
			Protocol: "https",
		}

		for range rule.HTTP.Paths {
			location := Location{
				Path:     "/",
				Upstream: name,

				// TODO: These need to be configurable
				SecureUpstream: false,
				SkipVerify:     true,
			}
			https.Locations = append(http.Locations, &location)
		}
		nginx.Listeners = append(nginx.Listeners, &https)

		log.Debug(fmt.Sprintf(
			"Creating NGINX config file with upstreams %+v and %+v, and listener %+v",
			nginx.UpstreamPlain,
			nginx.UpstreamSecure,
			nginx.Listeners))

		file, err := os.Create(filepath.Join(directory, name+".conf"))
		if err != nil {
			log.Error("Failed to create config file - ", err)
			return
		}

		if err = tplNginxConf.Execute(file, struct {
			Nginx *Nginx
		}{
			Nginx: &nginx,
		}); err != nil {
			log.Error("Failed to create template file - ", err)
		}

		file.Close()
	}
	reloadNginx()
}

// DeleteNginxConfig : Deletes a file from nginx config directory
func DeleteNginxConfig(name string) {
	var (
		re  *regexp.Regexp
		err error
	)

	if re, err = regexp.Compile("[.]*/?"); err != nil {
		return
	}

	// sanitize name to prevent directory traversal
	name = re.ReplaceAllString(name, "")
	if len(name) == 0 {
		return
	}

	var filename string = "/etc/nginx/sites.d/" + name + ".conf"
	info, err := os.Stat(filename)
	if os.IsNotExist(err) || info.IsDir() {
		return
	}

	os.Remove(filename)
	reloadNginx()
}

// reloadNginx : Reloads the NGINX systemd service
func reloadNginx() {
	var (
		err     error
		systemd *dbus.Conn
	)
	if nginxServer == nil {
		if systemd, err = dbus.NewSystemdConnection(); err != nil {
			log.Error("Failed to create dbus connection - ", err)
			return
		}
		nginxServer = &NginxServer{
			Systemd: systemd,
		}
	}

	nginxServer.Lock()
	defer nginxServer.Unlock()
	channel := make(chan string)
	_, err = nginxServer.Systemd.ReloadUnit("nginx.service", "replace", channel)
	if err != nil {
		log.Error("Failed to restart nginx service: ", err)
	}
	log.Info(<-channel)
}
