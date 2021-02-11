// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package pipeline : execution modelling
// The pipeline package offers mappings from JointJS Diagrams into golang
// for the purposes of designing a simplified overview of what a kubernetes
// deployment might look like.
package pipeline

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/notapipeline/tiyo/config"
	log "github.com/sirupsen/logrus"
)

// Pipeline : The principle interface for mapping JointJS to Kubernetes
type Pipeline struct {

	// The name of the pipeline as specified by the user
	Name string

	// DNS name is a DNS formatted identifier of the pipeline
	DNSName string

	// The Fully qualified domain name of the pipeline
	Fqdn string

	// How the pipeline is represented in storeage
	BucketName string

	// A list of container group objects
	Containers map[string]*Container

	// Commands which become docker containers in their own right
	Commands map[string]*Command

	// Links between the commands
	Links map[string]*LinkInterface

	// A set of source objects
	Sources map[string]*Source

	// Tiyo config object
	Config *config.Config

	// Global environment settings
	Environment []string

	// Global pipeline credentials (encrypted)
	Credentials map[string]string
}

// GetParent : Gets the parent (if any) of the current command element
// returns nil if not found
func (pipeline *Pipeline) GetParent(command *Command) *Container {
	var id string = command.Parent
	if _, ok := pipeline.Containers[id]; !ok {
		return nil
	}
	return pipeline.Containers[id]
}

// GetCommand : Gets the command at a given ID
// returns nil if not found
func (pipeline *Pipeline) GetCommand(id string) *Command {
	if _, ok := pipeline.Commands[id]; !ok {
		return nil
	}
	return pipeline.Commands[id]
}

// GetLink : Get the link at a given ID
// returns nil if not found
func (pipeline *Pipeline) GetLink(id string) *LinkInterface {
	if _, ok := pipeline.Links[id]; !ok {
		return nil
	}
	return pipeline.Links[id]
}

// GetLinksTo : Get all links feeding into a given command
// returns a slice of type *LinkInterface
func (pipeline *Pipeline) GetLinksTo(command *Command) []*LinkInterface {
	links := make([]*LinkInterface, 0)
	for _, link := range pipeline.Links {
		if (*link).GetLink().Target == command.ID {
			log.Debug("Link is target ", *link)
			links = append(links, link)
		}
	}
	return links
}

// GetLinksFrom : Get all links leading from a given command
// Return slice of type *LinkInterface
func (pipeline *Pipeline) GetLinksFrom(command *Command) []*LinkInterface {
	links := make([]*LinkInterface, 0)
	for _, link := range pipeline.Links {
		if (*link).GetLink().Source == command.ID {
			log.Debug("Link is source ", *link)
			links = append(links, link)
		}
	}
	return links
}

// GetStartIds : Gets a list of all IDs which have no inputs from other Command elements
// return slice of type string
func (pipeline *Pipeline) GetStartIds() []string {
	linkSources := make([]string, 0)
	startingPoints := make([]string, 0)

	for _, link := range pipeline.Links {
		// Ignore any link whose source is not a Command type.
		// these tend to be feeds rather than pipeline executables
		if _, ok := pipeline.Commands[(*link).GetLink().Source]; !ok {
			continue
		}
		// Any commands which do not have a link connected to the target
		// can be considered the start of the pipeline.
		// This allows to have multiple "starts" without defining
		// a specific object to cover that point.
		linkSources = append(linkSources, (*link).GetLink().Target)
	}

	for k := range pipeline.Commands {
		var found bool = false
		for _, id := range linkSources {
			if k == id {
				found = true
			}
		}
		if !found {
			startingPoints = append(startingPoints, k)
		}
	}
	return startingPoints
}

// GetStart : Gets all starting point commands
//
// Starting commands are commands which either have no
// links leading into them, or their links have a source whose type is
// not a Command struct.
//
// Starting commands will generally pick up directly off the event queue
// or require special case handling to initiate flow through the system
//
// return []*Command
func (pipeline *Pipeline) GetStart() []*Command {
	log.Debug("Getting commands at start of pipeline")
	ids := pipeline.GetStartIds()
	commands := make([]*Command, 0)
	for _, id := range ids {
		commands = append(commands, pipeline.Commands[id])
	}
	log.Debug(commands)
	return commands
}

// GetEndIds : Get all command ids at the end of the pipeline
//
// return []string
func (pipeline *Pipeline) GetEndIds() []string {
	linkSources := make([]string, 0)
	endPoints := make([]string, 0)

	for _, link := range pipeline.Links {
		// Any commands which are not a source of information
		// can be considered the end of the pipeline.
		// This allows to have multiple "starts" without defining
		// a specific object to cover that point.
		linkSources = append(linkSources, (*link).GetLink().Source)
	}

	for k := range pipeline.Commands {
		var found bool = false
		for _, id := range linkSources {
			if k == id {
				found = true
			}
		}
		if !found {
			endPoints = append(endPoints, k)
			continue
		}
	}
	return endPoints
}

// GetNextID : Get all IDs of commands following the present command
//
// return []string slice of IDs
func (pipeline *Pipeline) GetNextID(after *Command) []string {
	targets := make([]string, 0)
	for _, link := range pipeline.Links {
		if (*link).GetLink().Source == after.ID {
			targets = append(targets, (*link).GetLink().Target)
		}
	}
	return targets
}

// GetNext : Get the next command[s] following the present command
// return []*Command
func (pipeline *Pipeline) GetNext(after *Command) []*Command {
	var nextIDs = pipeline.GetNextID(after)
	targets := make([]*Command, 0)
	for i := 0; i < len(nextIDs); i++ {
		targets = append(targets, pipeline.Commands[nextIDs[i]])
	}
	return targets
}

// GetPreviousID : Get the previous ID[s]
// return []string
func (pipeline *Pipeline) GetPreviousID(before *Command) []string {
	sources := make([]string, 0)
	for _, link := range pipeline.Links {
		if (*link).GetLink().Target == before.ID {
			sources = append(sources, (*link).GetLink().Source)
		}
	}
	return sources
}

// GetPrev : Get previous command[s]
// return []*Command
func (pipeline *Pipeline) GetPrev(before *Command) []*Command {
	var priorIds = pipeline.GetPreviousID(before)
	sources := make([]*Command, 0)
	for i := 0; i < len(priorIds); i++ {
		if _, ok := pipeline.Commands[priorIds[i]]; ok {
			sources = append(sources, pipeline.Commands[priorIds[i]])
		}
	}
	return sources
}

// IsConvergence : Is this path a convergence point
//
// A convergence point is where multiple paths come together
// into a single command. Such points are often bottlenecks
// for data-flow, or offer services such as API or storage
// Under normal flow, a convergence point only runs a single
// instance and may cause the pipeline to wait for feed paths
// to complete before continuing.
func (pipeline *Pipeline) IsConvergence(command *Command) bool {
	return len(pipeline.GetPreviousID(command)) > 1
}

// GetConnection : Get the link between two instances
func (pipeline *Pipeline) GetConnection(source *Command, dest *Command) *LinkInterface {
	for _, link := range pipeline.Links {
		if (*link).GetLink().Source == source.ID && (*link).GetLink().Target == dest.ID {
			return link
		}
	}
	return nil
}

// Matcher : A container for regex matches
type Matcher struct {
	Source  string
	Pattern *regexp.Regexp
}

// linkSources : Get the source of a set of links
func (pipeline *Pipeline) linkSources(links []*LinkInterface) []Matcher {
	matchers := make([]Matcher, 0)
	all, _ := regexp.Compile(".*")
	if len(links) == 0 {
		matchers = append(matchers, Matcher{"", all})
	}

	for _, link := range links {
		if (*link).GetLink().Type == "file" {
			match := Matcher{}
			var path string = (*link).(*PathLink).Path
			if path == "" || (*link).(*PathLink).Path == pipeline.BucketName {
				sourceID := (*link).GetLink().Source
				if _, ok := pipeline.Commands[sourceID]; ok {
					path = pipeline.Commands[sourceID].Name
				} else if _, ok := pipeline.Sources[sourceID]; ok {
					path = pipeline.Sources[sourceID].Name
				}
				if path == pipeline.BucketName {
					// empty path if we match bucket name
					// as this will set it to the pipeline
					// root directory.
					path = ""
				}
			}
			log.Debug("Found path ", path, (*link))
			match.Source = path
			pattern, err := regexp.Compile((*link).(*PathLink).Pattern)
			if err != nil {
				log.Error("Cannot compile regex from pattern '.*' ", err, " using .* instead")
				pattern = all
			}

			if pattern == nil {
				pattern = all
			}
			match.Pattern = pattern
			matchers = append(matchers, match)
		}
	}
	matchers = pipeline.Unique(matchers)
	log.Debug("Got matches ", matchers)
	return matchers
}

// Unique : Gets a unique list of Match items
func (pipeline *Pipeline) Unique(matchers []Matcher) []Matcher {
	keys := make(map[string]bool)
	list := make([]Matcher, 0)

	for _, value := range matchers {
		key := value.Source + value.Pattern.String()
		if _, ok := keys[key]; !ok {
			keys[key] = true
			list = append(list, value)
		}
	}

	return list
}

// GetPathSources :
func (pipeline *Pipeline) GetPathSources(source *Command) []string {
	links := pipeline.GetLinksTo(source)
	sources := make([]string, 0)
	matches := pipeline.linkSources(links)
	for _, match := range matches {
		sources = append(sources, match.Source)
	}
	return sources
}

// WatchItems : Gets a list of directories/files to watch for events.
// Does not create.
//
// If path is empty, takes the name of the upstream command
func (pipeline *Pipeline) WatchItems() []Matcher {
	watch := make([]Matcher, 0)
	links := make([]*LinkInterface, 0)

	for _, link := range pipeline.Links {
		if (*link).(*PathLink).Watch {
			links = append(links, link)
		}
	}
	sources := pipeline.linkSources(links)
	watch = append(watch, sources...)
	return watch
}

// CommandFromContainerName : Get a command id from its final image name
func (pipeline *Pipeline) CommandFromContainerName(kubernetesGroup string, image string) *Command {
	var instances []*Command
	for _, group := range pipeline.Containers {
		if strings.HasSuffix(kubernetesGroup, group.Name) {
			instances = group.GetChildren()
		}
	}

	for _, command := range instances {
		if command.Name == image {
			return command
		}
	}
	return nil
}

// ContainerFromServiceName : Get a container for a given kubernetes service
func (pipeline *Pipeline) ContainerFromServiceName(serviceName string) *Container {
	for _, group := range pipeline.Containers {
		if strings.HasSuffix(serviceName, group.Name) {
			return group
		}
	}
	return nil
}

// GetPipeline : Load a pipeline by name from the bolt store and return a new pipeline
func GetPipeline(config *config.Config, name string) (*Pipeline, error) {
	pipeline := Pipeline{}
	pipeline.Name = name
	pipeline.DNSName = Sanitize(name, "-")
	pipeline.Fqdn = pipeline.DNSName + "." + config.DNSName
	pipeline.BucketName = Sanitize(name, "_")
	pipeline.Commands = make(map[string]*Command)
	pipeline.Links = make(map[string]*LinkInterface)
	pipeline.Containers = make(map[string]*Container)
	pipeline.Sources = make(map[string]*Source)
	pipeline.Config = config
	pipeline.Environment = make([]string, 0)
	pipeline.Credentials = make(map[string]string)

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: config.UseInsecureTLS}
	// Do not use pipeline.Name here - that has been Sanitized and will not match
	response, err := http.Get(fmt.Sprintf("%s/api/v1/bucket/pipeline/%s", config.AssembleServer(), name))
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	message := struct {
		Code    int    `json:"code"`
		Result  string `json:"result"`
		Message string `json:"message"`
	}{}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &message)
	if err != nil {
		return nil, err
	}

	pipelineJSON, err := base64.StdEncoding.DecodeString(string(message.Message))
	if err != nil {
		return nil, err
	}

	var content map[string]interface{}
	err = json.Unmarshal(pipelineJSON, &content)
	if err != nil {
		return nil, err
	}

	if environment, ok := content["environment"]; ok {
		env := environment.([]interface{})
		pipeline.Environment = make([]string, 0)
		for _, item := range env {
			pipeline.Environment = append(pipeline.Environment, item.(string))
		}
	}

	if credentials, ok := content["credentials"]; ok {
		creds := credentials.(map[string]interface{})
		for key, value := range creds {
			pipeline.Credentials[key] = value.(string)
		}
	}

	// issue#18
	// When parsing the pipeline, any errors / missing required variables
	// should be sent back to the browser as a map of "id:[errors]"
	for _, c := range content["cells"].([]interface{}) {
		cell := c.(map[string]interface{})
		switch cell["type"].(string) {
		case "container.Container":
			command := NewCommand(cell)
			command.Image = pipeline.Config.Docker.Upstream + "/" + command.GetContainer(false)
			if command.Custom {
				command.Image = command.GetContainer(false)
			}
			command.Tag = pipeline.Config.Docker.Primary + "/" + command.GetContainer(true)
			if pipeline.Config.Docker.Registry != "" {
				command.Tag = pipeline.Config.Docker.Registry + "/" + command.Tag
			}
			pipeline.AddEnv(command)
			pipeline.Commands[command.ID] = command
		case "container.Kubernetes":
			container := NewContainer(&pipeline, cell)
			pipeline.Containers[container.ID] = container
		case "container.Source":
			source := NewSource(cell)
			pipeline.Sources[source.ID] = source
		case "link":
			link := NewLink(cell)
			switch link.(type) {
			case *PathLink:
				pipeline.Links[link.(*PathLink).ID] = &link
			case *PortLink:
				pipeline.Links[link.(*PortLink).ID] = &link
			}
		}
	}
	return &pipeline, nil
}

// AddEnv : Add the full pipeline environment to a command
// This is normally called by flow immediatly prior to the command
// being sent out to the container. This will help keep memory
// footprint smaller during standard execution when dealing with
// large pipeline environments and/or multiple pipelines on one flow
func (pipeline *Pipeline) AddEnv(command *Command) {
	command.Environment = append(command.Environment, pipeline.Environment...)
}
