package flow

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mproffitt/tiyo/config"
	"github.com/mproffitt/tiyo/pipeline"
)

type Flow struct {
	Name           string
	PipelineBucket string
	Config         *config.Config
	Pipeline       *pipeline.Pipeline
	Flags          *flag.FlagSet
}

func NewFlow() *Flow {
	flow := Flow{}
	return &flow
}

func (flow *Flow) Init() {
	flow.Name = os.Getenv("TIYO_PIPELINE")
	description := "The name of the pipeline to use"
	flow.Flags = flag.NewFlagSet("flow", flag.ExitOnError)
	flow.Flags.StringVar(&flow.Name, "p", flow.Name, description)
	flow.Flags.Parse(os.Args[2:])
	if flow.Name == "" {
		flow.Flags.Usage()
		os.Exit(1)
	}
	flow.PipelineBucket = strings.ToLower(strings.ReplaceAll(flow.Name, " ", "_"))
}

/**
 * Creates an instance of the command environment if one does not already exist
 */
func (flow *Flow) Create(instance *pipeline.Command) {
	var containerName string = instance.Name
	if !instance.Custom {
		if !strings.Contains(instance.Name, "/") {
			containerName = fmt.Sprintf("%s/%s", flow.Config.Docker.Upstream, instance.Name)
		}
	}

	var containerExists bool = false
	if instance.UseExisting {
		// Does an existing instance exist?

		// connect kubernetes api, scan for existing pod matching name in all namespaces
		// if pod exists, assign command to pod listener

		// if pod does not exist, has it previously been built?
		// e.g. curl https://registry.hub.docker.com/v1/repositories/choclab/[NAME]/tags

		// If it has not been built, build it:
	}

	if !containerExists {
		path := fmt.Sprintf("containers/%s", containerName)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Create container build directory and CD to it
			cwd, _ := os.Getwd()
			os.MkdirAll(path, os.ModeDir)
			os.Chdir(path)
			flow.WriteDockerfile(containerName, instance.Version)
			// skaffold build
			os.Chdir(cwd)
		}
	}

}

func (flow *Flow) WriteDockerfile(containerName string, containerVersion string) error {
	var name string = "Dockerfile"
	template := fmt.Sprintf(dockerTemplate, containerName, containerVersion, flow.Name)
	file, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("Failed to create Dockerfile for %s. %s", containerName, err)
	}
	defer file.Close()
	if _, err := file.WriteString(template); err != nil {
		return fmt.Errorf("Failed to write Dockerfile for %s. Error was: %s", name, err)
	}
	file.Sync()
	return nil
}

func (flow *Flow) Run() int {
	var (
		err error
	)
	flow.Config, err = config.NewConfig()
	if err != nil {
		fmt.Printf("Error loading config file: %s\n", err)
		return 1
	}

	flow.Pipeline, err = pipeline.GetPipeline(flow.Config, flow.Name)
	if err != nil {
		fmt.Printf("Error loading pipeline '%s' - %s", flow.Name, err)
		return 1
	}

	return 0
}
