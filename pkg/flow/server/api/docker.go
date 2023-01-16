package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/notapipeline/tiyo/pkg/config"
	"github.com/notapipeline/tiyo/pkg/docker"
	"github.com/notapipeline/tiyo/pkg/pipeline"
	log "github.com/sirupsen/logrus"
)

// Create : Creates a new docker container image if one is not already found in the library
func (flow *Flow) Create(instance *pipeline.Command) error {
	log.Info("flow - Creating new container instance for ", instance.Name, " ", instance.ID)

	var containerExists bool
	var err error
	containerExists, err = flow.Docker.ContainerExists(instance.Tag)
	if err != nil {
		return err
	}

	if containerExists && !flow.update {
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

// Cleanup : Delete any redundant files left over from building the container
func (flow *Flow) Cleanup(path string, owd string, err error) error {
	os.Chdir(owd)
	if e := os.RemoveAll(path); e != nil {
		log.Error("Failed to clean up %s - manual intervention required\n", path)
	}
	return err
}

// WriteDockerfile ; Writes the template dockerfile ready for building the container
func (flow *Flow) WriteDockerfile(instance *pipeline.Command) error {
	log.Info("Creating Dockerfile ", instance.Image)
	var name string = "Dockerfile"
	template := fmt.Sprintf(docker.TiyoTemplate, instance.Image)
	if instance.Language == "dockerfile" && instance.Custom {
		var (
			script []byte
			err    error
		)
		if script, err = base64.StdEncoding.DecodeString(instance.ScriptContent); err != nil {
			return err
		}
		template = string(script)
	}

	file, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create Dockerfile for %s. %s", instance.Name, err)
	}
	defer file.Close()
	if _, err := file.WriteString(template); err != nil {
		return fmt.Errorf("failed to write Dockerfile for %s. Error was: %s", name, err)
	}
	file.Sync()
	log.Debug("Dockerfile written: ", instance.Image)
	return nil
}

// CopyTiyoBinary : Tiyo embeds itself into the containers it build to run in Syphon mode.
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

// WriteConfig : Create a basic config for Syphon to communicate with the current flow
func (flow *Flow) WriteConfig() error {
	log.Info("Creating stub config for container wrap")
	path, _ := os.Getwd()
	host := config.Host{
		Hostname:     flow.Config.Flow.Hostname,
		Port:         flow.Config.Flow.Port,
		ClientSecure: flow.Config.Flow.Cacert != "" && flow.Config.Flow.Cakey != "",
	}
	config := struct {
		SequenceBaseDir string      `json:"sequenceBaseDir"`
		UseInsecureTLS  bool        `json:"skipVerify"`
		Flow            config.Host `json:"flow"`
		AppName         string      `json:"appname"`
	}{
		SequenceBaseDir: flow.Config.SequenceBaseDir,
		UseInsecureTLS:  flow.Config.UseInsecureTLS,
		Flow:            host,
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

// Clones a git repository for each container in the set that has a git repo described
// TODO: Rewrite. This should clone from inside the container after build
func (flow *Flow) checkout(containers []*pipeline.Command) {
	for _, container := range containers {
		if container.GitRepo.RepoURL == "" {
			continue
		}
		var path string = filepath.Join(
			flow.Config.SequenceBaseDir,
			flow.Config.Kubernetes.Volume,
			flow.Pipeline.BucketName,
			container.Name,
			"src",
		)
		var password string = container.GitRepo.Password
		if password == "" {
			if container.GitRepo.Username != "" {
				if _, ok := flow.Pipeline.Credentials[container.GitRepo.Username]; !ok {
					log.Error("No password supplied for repo ", container.GitRepo.RepoURL)
					return
				}
				password = flow.Pipeline.Credentials[container.GitRepo.Username]
			}
		}

		// There is no need to decrypt the password until it is requried.
		// This aids in keeping the app secure by not holding unencrypted
		// passwords in memory for longer than they absolutely need to be.
		options := make(map[string]string)
		if password != "" {
			passwordDecrypted, err := DecryptData([]byte(password), flow.Config.Flow.Passphrase)
			if err != nil {
				log.Error(err)
			}
			options["password"] = string(passwordDecrypted)
		}
		if err := container.GitRepo.Clone(path, options); err != nil {
			log.Error(err)
			return
		}
		container.GitRepo.Checkout()
	}
}
