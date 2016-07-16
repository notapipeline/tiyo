package pipeline

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/mproffitt/tiyo/config"
)

type Pipeline struct {
	Name     string
	Commands map[string]*Command
	Links    map[string]*LinkInterface
}

func (pipeline *Pipeline) GetCommand(id string) *Command {
	return pipeline.Commands[id]
}

func (pipeline *Pipeline) GetLink(id string) *LinkInterface {
	return pipeline.Links[id]
}

func (pipeline *Pipeline) GetLinksTo(command *Command) []*LinkInterface {
	links := make([]*LinkInterface, 0)
	for _, link := range pipeline.Links {
		if (*link).GetLink().Target == command.Id {
			links = append(links, link)
		}
	}
	return links
}

func (pipeline *Pipeline) GetLinksFrom(command *Command) []*LinkInterface {
	links := make([]*LinkInterface, 0)
	for _, link := range pipeline.Links {
		if (*link).GetLink().Source == command.Id {
			links = append(links, link)
		}
	}
	return links
}

// Gets a list of all IDs which have no inputs from other elements
func (pipeline *Pipeline) GetStart() []string {
	linkSources := make([]string, 0)
	startingPoints := make([]string, 0)

	for _, link := range pipeline.Links {
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

func (pipeline *Pipeline) GetEnd() []string {
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

func (pipeline *Pipeline) GetNextId(after *Command) []string {
	id := after.Id
	targets := make([]string, 0)
	for _, link := range pipeline.Links {
		if (*link).GetLink().Source == id {
			targets = append(targets, (*link).GetLink().Target)
		}
	}
	return targets
}

func (pipeline *Pipeline) GetNext(after *Command) []*Command {
	var nextIds = pipeline.GetNextId(after)
	targets := make([]*Command, 0)
	for i := 0; i < len(nextIds); i++ {
		targets = append(targets, pipeline.Commands[nextIds[i]])
	}
	return targets
}

func (pipeline *Pipeline) GetPreviousId(before *Command) []string {
	id := before.Id
	sources := make([]string, 0)
	for _, link := range pipeline.Links {
		if (*link).GetLink().Target == id {
			sources = append(sources, (*link).GetLink().Source)
		}
	}
	return sources
}

func (pipeline *Pipeline) GetPrev(before *Command) []*Command {
	var priorIds = pipeline.GetPreviousId(before)
	sources := make([]*Command, 0)
	for i := 0; i < len(priorIds); i++ {
		sources = append(sources, pipeline.Commands[priorIds[i]])
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

// Gets a list of directories/files to watch for events
// does not create
//
// If path is empty, takes the name of the upstream command
func (pipeline *Pipeline) WatchItems() []string {
	watch := make([]string, 0)
	for _, link := range pipeline.Links {
		if (*link).GetLink().Type == "file" && (*link).(*PathLink).Watch {
			path := (*link).(*PathLink).Path
			if path == "" {
				name := strings.ToLower(pipeline.Commands[(*link).GetLink().Source].Name)
				path = strings.ReplaceAll(name, " ", "_")
			}
			// if path is still empty, ignore
			if path != "" {
				watch = append(watch, path)
			}
		}
	}
	return watch
}

func GetPipeline(config *config.Config, name string) (*Pipeline, error) {
	pipeline := Pipeline{}
	pipeline.Name = name
	pipeline.Commands = make(map[string]*Command)
	pipeline.Links = make(map[string]*LinkInterface)

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

	for _, c := range content["cells"].([]interface{}) {
		cell := c.(map[string]interface{})
		switch cell["type"].(string) {
		case "container.Element":
			command := NewCommand(cell)
			pipeline.Commands[command.Id] = command
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
