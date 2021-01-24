package flow

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/choclab-net/tiyo/config"
	"github.com/choclab-net/tiyo/pipeline"
)

type Flow struct {
	Name        string
	Update      bool
	Config      *config.Config
	Pipeline    *pipeline.Pipeline
	Docker      *Docker
	Kubernetes  *Kubernetes
	Flags       *flag.FlagSet
	Queue       *Queue
	IsExecuting bool
	Api         *FlowApi
}

func NewFlow() *Flow {
	flow := Flow{}
	flow.Api = NewFlowApi(&flow)
	flow.IsExecuting = false
	return &flow
}

func (flow *Flow) Init() {
	log.Info("Initialising flow")
	flow.Name = os.Getenv("TIYO_PIPELINE")
	description := "The name of the pipeline to use"
	flow.Flags = flag.NewFlagSet("flow", flag.ExitOnError)
	flow.Flags.StringVar(&flow.Name, "p", flow.Name, description)
	flow.Flags.BoolVar(&flow.Update, "u", false, "Update any containers")
	flow.Flags.Parse(os.Args[2:])
	log.Debug("Flow initialised", flow)
}

/**
 * Creates an instance of the command environment if one does not already exist
 */
func (flow *Flow) Create(instance *pipeline.Command) error {
	log.Info("flow - Creating new container instance for ", instance.Name, " ", instance.Id)
	var err error
	var containerExists bool = false
	containerExists, err = flow.Docker.ContainerExists(instance.Tag)
	if err != nil {
		return err
	}

	if containerExists && !flow.Update {
		log.Info("Not building image for ", instance.Image, " Image exists")
		return nil
	}

	path := fmt.Sprintf("containers/%s", instance.Tag)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create container build directory and CD to it
		owd, _ := os.Getwd()
		os.MkdirAll(path, 0775)
		os.Chdir(path)
		log.Debug("Changing to build path", path)
		if err := flow.WriteDockerfile(instance); err != nil {
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
	if e := os.RemoveAll(path); e != nil {
		log.Error("Failed to clean up %s - manual intervention required\n", path)
	}
	return err
}

func (flow *Flow) WriteDockerfile(instance *pipeline.Command) error {
	log.Info("Creating Dockerfile ", instance.Image)
	var name string = "Dockerfile"
	template := fmt.Sprintf(dockerTemplate, instance.Image)
	if instance.Language == "dockerfile" && instance.Custom {
		template = instance.ScriptContent
	}

	file, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("Failed to create Dockerfile for %s. %s", instance.Name, err)
	}
	defer file.Close()
	if _, err := file.WriteString(template); err != nil {
		return fmt.Errorf("Failed to write Dockerfile for %s. Error was: %s", name, err)
	}
	file.Sync()
	log.Debug("Dockerfile written: ", instance.Image)
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
	path, _ := os.Getwd()
	config := struct {
		SequenceBaseDir string      `json:"sequenceBaseDir"`
		UseInsecureTLS  bool        `json:"skipVerify"`
		Flow            config.Host `json:"flow"`
		AppName         string      `json:"appname"`
	}{
		SequenceBaseDir: flow.Config.SequenceBaseDir,
		UseInsecureTLS:  flow.Config.UseInsecureTLS,
		Flow:            flow.Config.Flow,
		AppName:         filepath.Base(path),
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

func (flow *Flow) Setup(pipelineName string) bool {
	flow.Name = pipelineName
	var err error

	// Load the pipeline
	flow.Pipeline, err = pipeline.GetPipeline(flow.Config, flow.Name)
	if err != nil {
		log.Error("issue loading pipeline ", flow.Name, " - ", err)
		return false
	}

	// Create the queue
	flow.Queue = NewQueue(flow.Config, flow.Pipeline, flow.Pipeline.BucketName)

	// create docker engine
	flow.Docker = NewDockerEngine(flow.Config)
	if err != nil {
		log.Error(err)
		return false
	}

	// create the Kubernetes engine
	flow.Kubernetes, err = NewKubernetes(flow.Config, flow.Pipeline)
	if err != nil {
		log.Error(err)
		return false
	}
	return true
}

func (flow *Flow) Execute() {
	flow.IsExecuting = true
	// create all missing containers
	for _, command := range flow.Pipeline.Commands {
		log.Debug("Pipeline start item", command)
		err := flow.Create(command)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Create the pipeline runtime engine
	// Each of these needs a level of error reporting enabling
	// other than "panic"
	for _, item := range flow.Pipeline.Containers {
		switch item.SetType {
		case "statefulset":
			go flow.Kubernetes.CreateStatefulSet(flow.Pipeline.DnsName, item)
		case "deployment":
			go flow.Kubernetes.CreateDeployment(flow.Pipeline.DnsName, item)
		case "daemonset":
			go flow.Kubernetes.CreateDaemonSet(flow.Pipeline.DnsName, item)
		}
	}
}

func (flow *Flow) Stop() {
	flow.IsExecuting = false
	flow.Queue.Stop()
}

func (flow *Flow) Start() {
	flow.IsExecuting = true
	flow.Queue.Start()
}

func (flow *Flow) Destroy() {
	flow.Stop()
	for _, item := range flow.Pipeline.Containers {
		switch item.SetType {
		case "statefulset":
			go flow.Kubernetes.DestroyStatefulSet(flow.Pipeline.DnsName, item)
		case "deployment":
			go flow.Kubernetes.DestroyDeployment(flow.Pipeline.DnsName, item)
		case "daemonset":
			go flow.Kubernetes.DestroyDaemonSet(flow.Pipeline.DnsName, item)
		}
	}
}

func (flow *Flow) Run() int {
	var (
		err error
	)
	log.Info("Starting flow executor")

	sigc := make(chan os.Signal, 1)
	done := make(chan bool)

	signal.Notify(sigc, os.Interrupt)
	go func() {
		for range sigc {
			log.Info("Shutting down listener")
			done <- true
		}
	}()

	flow.Config, err = config.NewConfig()
	if err != nil {
		log.Error("issue loading config file: ", err)
		return 1
	}

	log.Info("Setting working directory to ", flow.Config.DbDir)
	os.Chdir(flow.Config.DbDir)
	// Start server in background
	go flow.Api.Serve()
	if flow.Name != "" {
		flow.Setup(flow.Name)
		flow.Execute()
	}
	<-done
	log.Info("Flow complete")
	return 0
}
