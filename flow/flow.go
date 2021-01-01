package flow

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/choclab-net/tiyo/config"
	"github.com/choclab-net/tiyo/pipeline"
)

type Flow struct {
	Name           string
	PipelineBucket string
	Config         *config.Config
	Pipeline       *pipeline.Pipeline
	Docker         *Docker
	Kubernetes     *Kubernetes
	Flags          *flag.FlagSet
}

func NewFlow() *Flow {
	flow := Flow{}
	return &flow
}

func (flow *Flow) Init() {
	log.Info("Initialising flow")
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
	log.Debug("Flow initialised", flow)
}

/**
 * Creates an instance of the command environment if one does not already exist
 */
func (flow *Flow) Create(instance *pipeline.Command) error {
	log.Info("flow - Creating new container instance for ", instance.Name, " ", instance.Id)
	var containerName string = instance.Name
	if !instance.Custom {
		if !strings.Contains(instance.Name, "/") {
			containerName = fmt.Sprintf("%s/%s", flow.Config.Docker.Upstream, instance.Name)
		}
	}

	var err error
	var containerExists bool = false
	containerExists, err = flow.Docker.ContainerExists(instance.Image)
	if err != nil {
		return err
	}

	if containerExists {
		log.Info("Not building image for ", instance.Name, ":", instance.Version, " Image exists")
		return nil
	}

	path := fmt.Sprintf("containers/%s", containerName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create container build directory and CD to it
		owd, _ := os.Getwd()
		os.MkdirAll(path, 0775)
		os.Chdir(path)
		log.Debug("Changing to build path", path)
		if err := flow.WriteDockerfile(containerName, instance.Version); err != nil {
			return flow.Cleanup(path, owd, err)
		}

		if err := flow.CopyTiyoBinary(); err != nil {
			return flow.Cleanup(path, owd, err)
		}

		if err := flow.WriteConfig(); err != nil {
			return flow.Cleanup(path, owd, err)
		}
		err = flow.Docker.Create(instance)
		if err != nil {
			return flow.Cleanup(path, owd, err)
		}
		flow.Cleanup(path, owd, nil)
	}
	return nil
}

func (flow *Flow) Cleanup(path string, owd string, err error) error {
	os.Chdir(owd)
	if err := os.RemoveAll(path); err != nil {
		log.Error("Failed to clean up %s - manual intervention required\n", path)
	}
	return err
}

func (flow *Flow) WriteDockerfile(containerName string, containerVersion string) error {
	log.Debug("Creating Dockerfile", containerName, containerVersion)
	var name string = "Dockerfile"
	template := fmt.Sprintf(dockerTemplate, containerName, containerVersion)
	file, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("Failed to create Dockerfile for %s. %s", containerName, err)
	}
	defer file.Close()
	if _, err := file.WriteString(template); err != nil {
		return fmt.Errorf("Failed to write Dockerfile for %s. Error was: %s", name, err)
	}
	file.Sync()
	log.Debug("Dockerfile written:", containerName, containerVersion)
	return nil
}

func (flow *Flow) CopyTiyoBinary() error {
	log.Debug("Copying tiyo binary")

	path, err := os.Executable()
	if err != nil {
		return err
	}
	sourceFileStat, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}

	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(filepath.Base(path))
	if err != nil {
		return err
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return err
	}
	return nil
}

func (flow *Flow) WriteConfig() error {
	log.Debug("Creating stub config for container wrap")
	config := struct {
		SequenceBaseDir string      `json:"sequenceBaseDir"`
		UseInsecureTLS  bool        `json:"skip_verify"`
		Assemble        config.Host `json:"assemble"`
	}{
		SequenceBaseDir: flow.Config.SequenceBaseDir,
		UseInsecureTLS:  flow.Config.UseInsecureTLS,
		Assemble:        flow.Config.Assemble,
	}
	bytes, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile("config.json", bytes, 0644); err != nil {
		return err
	}
	return nil
}

func (flow *Flow) Run() int {
	var (
		err error
	)
	log.Info("Starting flow executor")

	flow.Config, err = config.NewConfig()
	if err != nil {
		log.Error("issue loading config file: %s\n", err)
		return 1
	}

	flow.Pipeline, err = pipeline.GetPipeline(flow.Config, flow.Name)
	if err != nil {
		log.Error("issue loading pipeline '%s' - %s", flow.Name, err)
		return 1
	}

	flow.Docker = NewDockerEngine(flow.Config)
	if err != nil {
		log.Error(err)
		return 1
	}

	start := flow.Pipeline.GetStart()
	for _, command := range start {
		log.Debug("Pipeline start item", command)
		err := flow.Create(command)
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Info("Flow complete")
	return 0
}
