// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package flow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/notapipeline/tiyo/config"
	"github.com/coreos/go-systemd/dbus"
	log "github.com/sirupsen/logrus"
	networkv1 "k8s.io/api/networking/v1"
)

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

	// The upstream to build
	Upstream *Upstream

	// A listener object to listen against
	Listener *Listener

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
		ups := Upstream{}
		ups.Name = name
		ups.Options = []string{"least_conn;"}
		addresses := make([]string, 0)
		for _, addr := range *upstream {
			log.Debug(fmt.Sprintf("Found service port %+v", addr))
			addresses = append(addresses, fmt.Sprintf("%s:%d", addr.Address, addr.Port))
		}
		ups.Addresses = addresses

		// basic first round listener
		http := Listener{
			Listen:   80,
			Hostname: ups.Name + "." + rule.Host,
			Domain:   config.DNSName,
			Protocol: "http",
		}
		for range rule.HTTP.Paths {
			location := Location{
				Path:     "/",
				Upstream: name,
			}
			http.Locations = append(http.Locations, &location)
		}

		nginx := Nginx{
			Upstream: &ups,
			Listener: &http,
		}

		log.Debug(fmt.Sprintf(
			"Creating NGINX config file with upstream %+v and listener %+v",
			nginx.Upstream,
			nginx.Listener))
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

// CreateSSLCertificates : Create self signed certificates for the NGINX server
//
// If real certificates are required, you can safely replace the self signed ones
// with real certificates sharing the same name.
//
// It is unwise to change the configuration file directly as tiyo will have no
// knowledge of your changes and will replace the configuration if the pipeline is
// rebuilt for any reason.
func CreateSSLCertificates(hostname string) {}
