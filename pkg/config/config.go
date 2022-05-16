// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package config defines the primary configuration structure loaded from
// JSON configuration either in the current working directory or in
// `/etc/tiyo/tiyo.json`
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// TIMEOUT : Default timeout for http requests
const (
	TIMEOUT             time.Duration = 5 * time.Second
	SESSION_COOKIE_NAME string        = "__tiyo_session"
	SSO_SESSION_NAME    string        = "__tiyo_sso_session"
)

var Designate string = ""

// Host : Define how a host should be configured
//
// A host is one of `assemble` or `flow` and will contain
// information on how to start the host.
// If Cacert and CaKey are defined and not empty, the host
// will start on an SSL encrypted channel. This is the
// recommended behaviour in all instances, particularly for
// the assemble server which encrypts user provided passwords
// over the wire.
type Host struct {

	// The hostname to run the server on
	Host string `json:"host"`

	// The port to listen on. For assemble, the default is 8180
	// and for flow the default is 8280.
	Port int `json:"port"`

	// An optional certificate to encrypt traffic into the host
	Cacert string `json:"cacert,omitempty"`

	// An optional certificate key - mutually inclusive with Cacert
	Cakey string `json:"cakey,omitempty"`

	// A passphrase to encrypt user provided credentials
	//
	// For assemble, this should be a secure passphrase, normally
	// generated as the output of `pwgen -synr \`\"\\ 20 1`
	//
	// For flow, this should be the encrypted version of the same
	// password which can be generated by running `tiyo encrypt primary`
	// after completing the assemble config
	Passphrase string `json:"passphrase,omitempty"`

	// ClientSecure - syphon required switch for http(s)
	ClientSecure bool `json:"secure,omitempty"`
}

// Kubernetes : Define the connection to the kubernetes cluster
type Kubernetes struct {

	// A path to the kubernetes cluster config to use
	ConfigFile string `json:"kubeconfig"`

	// The principle namespace to deploy into
	Namespace string `json:"namespace"`

	// The data-volume to mount
	Volume string `json:"volume"`
}

// Docker : Configiration for the Docker client
type Docker struct {

	// Docker registry to use. Default for this is dockerhub
	Registry string `json:"registry"`

	// The username to log in to the docker registry with
	Username string `json:"username"`

	// Api token to authenticate against the registry
	Token string `json:"token"`

	// Principle location for upstream containers.
	//
	// When defined, this will be used as a source for listing
	// containers in the `applications` sidebar, and a primary
	// source for all vanilla containers.
	Upstream string `json:"upstream"`

	// The location to store all containers built by the tiyo
	// flow server. Most containers in this location would normally
	// include `tiyo syphon` as their `ps 1`
	Primary string `json:"primary"`

	// Set to true if both primary and upstream are the same location
	SameSource bool `default:"false"`
}

// Config : Primary configuration object
type Config struct {

	// Defines the primary location on the fileserver
	// for files to be stored
	SequenceBaseDir string `json:"sequenceBaseDir"`

	// If true, will configure an nginx server running in
	// the same location as flow server for access to services
	// running inside the cluster.
	ExternalNginx bool `json:"externalNginx"`

	// The name of the database file
	Dbname string `json:"dbname"`

	// If true will skip certificate checking
	UseInsecureTLS bool `json:"skipVerify"`

	// Host configuration for the assemble server
	Assemble Host `json:"assemble"`

	// Host configuration for the flow server
	Flow Host `json:"flow"`

	// Kubernetes configuration
	Kubernetes Kubernetes `json:"kubernetes"`

	// Docker configuration
	Docker Docker `json:"docker"`

	// AppName for testing syphon locally
	AppName string `json:"appname"`

	// Primary DNS name for services
	DNSName string `json:"dnsName"`

	// Config for SAML 2fa
	SAML *SAML `json:"saml"`

	// Base directory for configuration files - default /etc/tiyo
	ConfigBase string

	// Base directory for the database and container creation - default /var/tiyo
	DbDir string

	// Timeout - constant TIMEOUT
	TIMEOUT time.Duration
}

// NewConfig : Create a new configuration object and load the config file
func NewConfig() (*Config, error) {
	config := Config{
		DNSName:    "example.com",
		ConfigBase: "/etc/tiyo",
		DbDir:      "/var/tiyo",
		TIMEOUT:    TIMEOUT,
	}
	var (
		err        error
		configfile string = "tiyo.json"
	)
	_, err = os.Stat(configfile)
	if os.IsNotExist(err) {
		configfile = config.ConfigBase + "/" + configfile
	}

	log.Info("Using config file: ", configfile)
	jsonFile, err := os.Open(configfile)
	if err != nil {
		return nil, err
	}

	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	json.Unmarshal([]byte(byteValue), &config)

	// Allow assemble to be run at root
	if config.Assemble.Host == "" {
		config.Assemble.Host = config.DNSName
	} else if len(strings.Split(config.Assemble.Host, ".")) == 1 {
		config.Assemble.Host = config.Assemble.Host + "." + config.DNSName
	}

	if len(strings.Split(config.Flow.Host, ".")) == 1 {
		config.Flow.Host = config.Flow.Host + "." + config.DNSName
	}

	if config.Docker.Upstream == "" && config.Docker.Primary != "" {
		config.Docker.Upstream = config.Docker.Primary
	} else if config.Docker.Primary == "" && config.Docker.Upstream != "" {
		config.Docker.Primary = config.Docker.Upstream
	}

	if config.Docker.Upstream == config.Docker.Primary {
		config.Docker.SameSource = true
	}

	if config.Kubernetes.Namespace == "" {
		config.Kubernetes.Namespace = "default"
	}

	if config.Kubernetes.ConfigFile == "" {
		config.Kubernetes.ConfigFile = "config"
	}

	if config.SAML == nil {
		config.SAML = &SAML{}
	}

	var home string
	if home, err = os.UserHomeDir(); err != nil {
		// assume running as root
		home = "/root"
	}

	// try to load config in order
	// - path defined in config
	// - /etc/tiyo/{config}
	// - ~/.kube/{config}
	if _, err = os.Stat(config.Kubernetes.ConfigFile); os.IsNotExist(err) {
		var kubeconfig = config.ConfigBase + "/" + config.Kubernetes.ConfigFile
		if _, err = os.Stat(kubeconfig); os.IsNotExist(err) {
			config.Kubernetes.ConfigFile = home + "/.kube/" + config.Kubernetes.ConfigFile
		} else {
			config.Kubernetes.ConfigFile = kubeconfig
		}

		// don't panic if we're not flow, we don't need kubeconfig
		if _, err = os.Stat(config.Kubernetes.ConfigFile); os.IsNotExist(err) && Designate == "flow" {
			return nil, err
		}
	}

	return &config, nil
}

// AssembleServer : Get the address of the assemble server
func (config *Config) AssembleServer() string {
	var protocol string = "http"
	if config.Assemble.Cacert != "" && config.Assemble.Cakey != "" {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s:%d", protocol, config.Assemble.Host, config.Assemble.Port)
}

// FlowServer : Get the address of the flow FlowServer
func (config *Config) FlowServer() string {
	var protocol string = "http"
	if config.Flow.ClientSecure || (config.Flow.Cacert != "" && config.Flow.Cakey != "") {
		protocol = "https"
	}
	host := fmt.Sprintf("%s://%s:%d", protocol, config.Flow.Host, config.Flow.Port)
	return host
}

// GetPassphrase : Get the server specific passphrase for encryption
//
// from string Whether to retrieve `assemble` or `flow` passphrases
//
// future, this will optionally read the encryption passphrase from vault
func (config *Config) GetPassphrase(from string) string {
	switch from {
	case "assemble":
		return config.Assemble.Passphrase
	case "flow":
		return config.Flow.Passphrase
	}
	return ""
}
