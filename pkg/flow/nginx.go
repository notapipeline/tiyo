// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package flow

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/coreos/go-systemd/dbus"
	"github.com/notapipeline/tiyo/pkg/config"
	log "github.com/sirupsen/logrus"
	networkv1 "k8s.io/api/networking/v1"
)

// KEYSIZE : Sets the keysize of the RSA algorithm
const KEYSIZE int = 2048

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
		ups := Upstream{}
		ups.Name = name
		ups.Options = []string{"least_conn;"}
		addresses := make([]string, 0)

		nginx := Nginx{
			Upstream: &ups,
		}
		nginx.Listeners = make([]*Listener, 0)

		for _, addr := range *upstream {
			log.Debug(fmt.Sprintf("Found service port %+v", addr))
			addresses = append(addresses, fmt.Sprintf("%s:%d", addr.Address, addr.Port))
		}

		ups.Addresses = addresses

		var hostname = ups.Name + "." + rule.Host

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
			}
			https.Locations = append(http.Locations, &location)
		}
		nginx.Listeners = append(nginx.Listeners, &https)

		log.Debug(fmt.Sprintf(
			"Creating NGINX config file with upstream %+v and listener %+v",
			nginx.Upstream,
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

// CreateSSLCertificates : Create self signed certificates for the NGINX server
//
// If real certificates are required, you can safely replace the self signed ones
// with real certificates sharing the same name.
//
// It is unwise to change the configuration file directly as tiyo will have no
// knowledge of your changes and will replace the configuration if the pipeline is
// rebuilt for any reason.
func CreateSSLCertificates(hostname string) error {
	key, err := rsa.GenerateKey(rand.Reader, KEYSIZE)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"hostname"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24 * 180),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	publicKey := pubkey(key)
	certificate, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey, key)
	if err != nil {
		log.Fatalf("Failed to create certificate: %s", err)
	}

	out := &bytes.Buffer{}
	pem.Encode(out, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificate,
	})

	if err := write(out.String(), "/etc/ssl/"+hostname+"/certificate.crt"); err != nil {
		log.Error("Failed to create certificate ", err)
	}

	out.Reset()

	pem.Encode(out, pemblock(key))
	if write(out.String(), "/etc/ssl/"+hostname+"/certificate.key"); err != nil {
		log.Error("Failed to create key ", err)
	}

	return nil
}

func pubkey(key interface{}) interface{} {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	}
	return nil
}

func pemblock(key interface{}) *pem.Block {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	}
	return nil
}

func write(what, where string) error {
	if _, err := os.Stat(where); err != nil {
		if _, err := os.Stat(filepath.Dir(where)); os.IsNotExist(err) {
			os.Mkdir(filepath.Dir(where), 0755)
		}

		file, err := os.Create(where)
		if err != nil {
			return fmt.Errorf("Failed to create %s. %s", where, err)
		}
		defer file.Close()
		if _, err := file.WriteString(what); err != nil {
			return fmt.Errorf("Failed to write ssh key contents for %s. Error was: %s", where, err)
		}
	}
	return nil
}
