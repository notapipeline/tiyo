package pipeline

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"

	"github.com/choclab-net/tiyo/config"
	log "github.com/sirupsen/logrus"
)

type Pipeline struct {
	// The name of the pipeline as specified by the user
	Name string

	// DNS name is a DNS formatted identifier of the pipeline
	DnsName string

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
}

// Gets the parent (if any) of the current command element
// returns nil if not found
func (pipeline *Pipeline) GetParent(command *Command) *Container {
	var id string = command.Parent
	if _, ok := pipeline.Containers[id]; !ok {
		return nil
	}
	return pipeline.Containers[id]
}

// Gets the command at a given ID
// returns nil if not found
func (pipeline *Pipeline) GetCommand(id string) *Command {
	if _, ok := pipeline.Commands[id]; !ok {
		return nil
	}
	return pipeline.Commands[id]
}

// Get the link at a given ID
// returns nil if not found
func (pipeline *Pipeline) GetLink(id string) *LinkInterface {
	if _, ok := pipeline.Links[id]; !ok {
		return nil
	}
	return pipeline.Links[id]
}

// Get all links feeding into a given command
// returns a slice of type *LinkInterface
func (pipeline *Pipeline) GetLinksTo(command *Command) []*LinkInterface {
	links := make([]*LinkInterface, 0)
	for _, link := range pipeline.Links {
		if (*link).GetLink().Target == command.Id {
			log.Debug("Link is target ", *link)
			links = append(links, link)
		}
	}
	return links
}

// Get all links leading from a given command
// Return slice of type *LinkInterface
func (pipeline *Pipeline) GetLinksFrom(command *Command) []*LinkInterface {
	links := make([]*LinkInterface, 0)
	for _, link := range pipeline.Links {
		if (*link).GetLink().Source == command.Id {
			log.Debug("Link is source ", *link)
			links = append(links, link)
		}
	}
	return links
}

// Gets a list of all IDs which have no inputs from other Command elements
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

// Gets all starting point commands
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

// Get all command ids at the end of the pipeline
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

// Get all IDs of commands following the present command
//
// return []string slice of IDs
func (pipeline *Pipeline) GetNextId(after *Command) []string {
	targets := make([]string, 0)
	for _, link := range pipeline.Links {
		if (*link).GetLink().Source == after.Id {
			targets = append(targets, (*link).GetLink().Target)
		}
	}
	return targets
}

// Get the next command[s] following the present command
// return []*Command
func (pipeline *Pipeline) GetNext(after *Command) []*Command {
	var nextIds = pipeline.GetNextId(after)
	targets := make([]*Command, 0)
	for i := 0; i < len(nextIds); i++ {
		targets = append(targets, pipeline.Commands[nextIds[i]])
	}
	return targets
}

// Get the previous ID[s]
// return []string
func (pipeline *Pipeline) GetPreviousId(before *Command) []string {
	sources := make([]string, 0)
	for _, link := range pipeline.Links {
		if (*link).GetLink().Target == before.Id {
			sources = append(sources, (*link).GetLink().Source)
		}
	}
	return sources
}

// Get previous command[s]
// return []*Command
func (pipeline *Pipeline) GetPrev(before *Command) []*Command {
	var priorIds = pipeline.GetPreviousId(before)
	sources := make([]*Command, 0)
	for i := 0; i < len(priorIds); i++ {
		if _, ok := pipeline.Commands[priorIds[i]]; ok {
			sources = append(sources, pipeline.Commands[priorIds[i]])
		}
	}
	return sources
}

// A convergence point is where multiple paths come together
// into a single command. Such points are often bottlenecks
// for data-flow, or offer services such as API or storage
// Under normal flow, a convergence point only runs a single
// instance and may cause the pipeline to wait for feed paths
// to complete before continuing.
func (pipeline *Pipeline) IsConvergence(command *Command) bool {
	return len(pipeline.GetPreviousId(command)) > 1
}

func (pipeline *Pipeline) GetConnection(source *Command, dest *Command) *LinkInterface {
	for _, link := range pipeline.Links {
		if (*link).GetLink().Source == source.Id && (*link).GetLink().Target == dest.Id {
			return link
		}
	}
	return nil
}

type Matcher struct {
	Source  string
	Pattern *regexp.Regexp
}

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
				sourceId := (*link).GetLink().Source
				if _, ok := pipeline.Commands[sourceId]; ok {
					path = pipeline.Commands[sourceId].Name
				} else if _, ok := pipeline.Sources[sourceId]; ok {
					path = pipeline.Sources[sourceId].Name
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

// Gets a unique list of Match items
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

func (pipeline *Pipeline) GetPathSources(source *Command) []string {
	links := pipeline.GetLinksTo(source)
	sources := make([]string, 0)
	matches := pipeline.linkSources(links)
	for _, match := range matches {
		sources = append(sources, match.Source)
	}
	return sources
}

// Gets a list of directories/files to watch for events.
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

// Get a command id from its final image name
func (pipeline *Pipeline) CommandFromImageName(image string) *Command {
	for _, command := range pipeline.Commands {
		tagname := pipeline.Config.Docker.Primary + "/" + command.GetContainer(true)
		if tagname == image {
			return command
		}
	}
	return nil
}

func GetPipeline(config *config.Config, name string) (*Pipeline, error) {
	pipeline := Pipeline{}
	pipeline.Name = name
	pipeline.DnsName = sanitize(name, "-")
	pipeline.Fqdn = pipeline.DnsName + "." + config.DnsName
	pipeline.BucketName = sanitize(name, "_")
	pipeline.Commands = make(map[string]*Command)
	pipeline.Links = make(map[string]*LinkInterface)
	pipeline.Containers = make(map[string]*Container)
	pipeline.Sources = make(map[string]*Source)
	pipeline.Config = config

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: config.UseInsecureTLS}
	// Do not use pipeline.Name here - that has been sanitized and will not match
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

	pipelineJson, err := base64.StdEncoding.DecodeString(string(message.Message))
	if err != nil {
		return nil, err
	}

	var content map[string]interface{}
	err = json.Unmarshal(pipelineJson, &content)
	if err != nil {
		return nil, err
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
			pipeline.Commands[command.Id] = command
		case "container.Kubernetes":
			container := NewContainer(&pipeline, cell)
			pipeline.Containers[container.Id] = container
		case "container.Source":
			source := NewSource(cell)
			pipeline.Sources[source.Id] = source
		case "link":
			link := NewLink(cell)
			switch link.(type) {
			case *PathLink:
				pipeline.Links[link.(*PathLink).Id] = &link
			case *PortLink:
				pipeline.Links[link.(*PortLink).Id] = &link
			}
		}
	}
	return &pipeline, nil
}
