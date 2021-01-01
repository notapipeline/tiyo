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
)

type Host struct {
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Cacert string `json:"cacert"`
	Cakey  string `json:"cakey"`
}

type Kubernetes struct {
	ConfigFile string `json:"kubeconfig"`
	Namespace  string `json:"namespace"`
}

type Docker struct {
	Registry   string `json:"registry"`
	Username   string `json:"username"` // Future - to come from Vault
	Token      string `json:"token"`    // Future - to come from Vault
	Upstream   string `json:"upstream"`
	Primary    string `json:"primary"`
	SameSource bool   `default:"false"`
}

type Config struct {
	SequenceBaseDir string     `json:"sequenceBaseDir"`
	Dbname          string     `json:"dbname"`
	UseInsecureTLS  bool       `json:"skip_verify"`
	Assemble        Host       `json:"assemble"`
	Flow            Host       `json:"flow"`
	Kubernetes      Kubernetes `json:"cluster"`
	Docker          Docker     `json:"docker"`
}

func NewConfig() (*Config, error) {
	config := Config{}
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

func (config *Config) SequenceDir() string {
	parts := []string{
		config.SequenceBaseDir,
	}
	return strings.Join(parts, "/")
}
