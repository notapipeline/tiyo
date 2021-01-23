/**
 * Pipeline config class
 */
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

type Host struct {
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Cacert string `json:"cacert,omitempty"`
	Cakey  string `json:"cakey,omitempty"`
}

type Kubernetes struct {
	ConfigFile string `json:"kubeconfig"`
	Namespace  string `json:"namespace"`
	Volume     string `json:"volume"`
}

type Docker struct {
	Registry   string `json:"registry"`
	Username   string `json:"username"`
	Token      string `json:"token"`
	Upstream   string `json:"upstream"`
	Primary    string `json:"primary"`
	SameSource bool   `default:"false"`
}

type Config struct {
	SequenceBaseDir string     `json:"sequenceBaseDir"`
	ExternalNginx   bool       `json:"externalNginx"`
	Dbname          string     `json:"dbname"`
	UseInsecureTLS  bool       `json:"skipVerify"`
	Assemble        Host       `json:"assemble"`
	Flow            Host       `json:"flow"`
	Kubernetes      Kubernetes `json:"kubernetes"`
	Docker          Docker     `json:"docker"`
	AppName         string     `json:"appname"`
	DnsName         string     `json:"dnsName"`
}

func NewConfig() (*Config, error) {
	config := Config{
		DnsName: "example.com",
	}
	jsonFile, err := os.Open("config.json")
	if err != nil {
		return nil, err
	}

	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	json.Unmarshal([]byte(byteValue), &config)

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

	return &config, nil
}

func (config *Config) AssembleServer() string {
	var protocol string = "http"
	if config.Assemble.Cacert != "" && config.Assemble.Cakey != "" {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s:%d", protocol, config.Assemble.Host, config.Assemble.Port)
}

func (config *Config) FlowServer() string {
	var protocol string = "http"
	if config.Flow.Cacert != "" && config.Flow.Cakey != "" {
		protocol = "https"
	}
	host := fmt.Sprintf("%s://%s:%d", protocol, config.Flow.Host, config.Flow.Port)
	log.Debug(host)
	return host
}

func (config *Config) SequenceDir() string {
	parts := []string{
		config.SequenceBaseDir,
	}
	return strings.Join(parts, "/")
}
