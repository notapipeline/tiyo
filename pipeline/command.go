// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// TIMEOUT : The maximum number of minutes a command can execute for if not forever
const TIMEOUT = 15

// Command : Define the structure of a command taken from JointJS and executed by Syphon
type Command struct {

	// ID is the jointJS element ID
	ID string `json:"id"`

	// Parent is the jointJS ID of a parent container
	Parent string `json:"parent"`

	// The command container name
	Name string `json:"name"`

	// The executable binary name to trigger when events are recieved
	Command string `json:"command"`

	// Should the command automatically be started
	AutoStart bool `json:"autostart"`

	// Arguments to give to the command
	Args string `json:"args"`

	// The version of the container
	Version string `json:"version"`

	// The programming language the container is executing
	Language string `json:"element"`

	// Does this container have a script to execute
	Script bool `json:"script"`

	// The content of the script - will be base64 encoded for transmission safety
	ScriptContent string `json:"scriptcontent"`

	// Is this a custom container or is it a pre-fabricated container from an upstream source
	Custom bool `json:"custom"`

	// The user defined timeout in minutes - if >15 will be rounded down to 15
	Timeout int `json:"timeout"`

	// (Re)Use an existing container
	UseExisting bool `json:"existing"`

	// Expose this port as a service port
	ExposePort int `json:"exposeport"`

	// Is the service port a UDP port
	IsUDP bool `json:"isudp"`

	// The required CPU of the container (default 500m cpu)
	CPU string `json:"cpu"`

	// The required memory of the container (default 256Mi)
	Memory string `json:"memory"`

	// The time this command was started in unix nano
	StartTime int64 `json:"starttime"`

	// The time this command ended in unix nano
	EndTime int64 `json:"endtime"`

	// Environment variables to set directly on the command
	Environment []string `json:"environment"`

	// The image string to build the docker container from and load into kubernetes
	Image string

	// The computed tag to upload the container docker image as
	Tag string

	// Standard output buffer
	Stdout bytes.Buffer

	// Standard error buffer
	Stderr bytes.Buffer

	// The commands process ID whilst executing
	ProcessID int

	// A computed set of process arguments including any files
	ProcessArgs []string

	// A computed file separator
	FileSeperator string
}

var regex *regexp.Regexp

// Sanitize a given string, removing any non-alphanumeric characters and replacing them with sep.
func Sanitize(str string, sep string) string {
	regex, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		log.Fatal("Failed to compile regex - ", err)
	}
	return strings.Trim(strings.ToLower(regex.ReplaceAllString(str, sep)), sep)
}

// NewCommand : Create a new command instance
func NewCommand(cell map[string]interface{}) *Command {
	command := Command{
		ID:            "",
		Name:          "",
		Command:       "",
		AutoStart:     false,
		Args:          "",
		Version:       "",
		Language:      "",
		Script:        false,
		ScriptContent: "",
		Custom:        false,
		Timeout:       TIMEOUT,
		UseExisting:   false,
		ExposePort:    -1,
		IsUDP:         false,
		StartTime:     0,
		EndTime:       0,
		CPU:           "500m",
		Memory:        "256Mi",
	}
	command.Environment = make([]string, 0)

	if cell["id"] != nil {
		command.ID = cell["id"].(string)
	}

	if cell["parent"] != nil {
		command.Parent = cell["parent"].(string)
	}

	if cell["name"] != nil {
		command.Name = Sanitize(cell["name"].(string), "-")
	}

	if cell["command"] != nil {
		command.Command = cell["command"].(string)
	}

	if cell["autostart"] != nil {
		command.AutoStart = cell["autostart"].(bool)
	}

	if cell["arguments"] != nil {
		command.Args = cell["arguments"].(string)
	}

	if cell["version"] != nil {
		command.Version = cell["version"].(string)
	}

	if cell["element"] != nil {
		command.Language = cell["element"].(string)
	}

	if cell["script"] != nil {
		command.Script = cell["script"].(bool)
	}

	if cell["scriptcontent"] != nil {
		command.ScriptContent = cell["scriptcontent"].(string)
	}

	if cell["custom"] != nil {
		command.Custom = cell["custom"].(bool)
	}

	if cell["timeout"] != nil {
		command.Timeout = int(cell["timeout"].(float64))
		if command.Timeout == 0 {
			command.Timeout = TIMEOUT
		}
		if command.Timeout != -1 {
			command.Timeout = command.Timeout * 60
		}
	} else {
		command.Timeout = TIMEOUT * 60
	}

	if cell["existing"] != nil {
		command.UseExisting = cell["existing"].(bool)
	}

	if cell["exposeport"] != nil {
		command.ExposePort = int(cell["exposeport"].(float64))
	}

	if cell["isudp"] != nil {
		command.IsUDP = cell["isudp"].(bool)
	}

	if cell["cpu"] != nil {
		command.CPU = cell["cpu"].(string)
	}

	if cell["memory"] != nil {
		command.Memory = cell["memory"].(string)
	}

	if cell["environment"] != nil {
		envvars := cell["environment"].([]interface{})
		for _, value := range envvars {
			command.Environment = append(command.Environment, value.(string))
		}
	}

	return &command
}

// AddEnvVar : Add a value to the environment variables
// key, value will be converted to KEY=value
func (command *Command) AddEnvVar(key, value string) {
	command.Environment = append(command.Environment, strings.ToUpper(key)+"='"+value+"'")
}

// GetContainer : Gets a container build string.
//
// if asTag is True, returns a modified form of the container string
// for uploading to the primary source
func (command *Command) GetContainer(asTag bool) string {
	container := command.Language
	switch command.Language {
	case "r":
		container = "r-base"
	case "javascript":
		container = "node"
	case "jupyter":
		container = "jupyter/datascience-notebook"
		if asTag {
			container = "datascience-notebook"
		}
	case "dockerfile":
		container = command.Name
	}

	var name string = container + ":" + command.Version
	if asTag {
		name = container + "-tiyo:" + command.Version
	}

	if !command.Custom {
		name = command.Name + ":" + command.Version
		if asTag {
			name = command.Name + "-tiyo:" + command.Version
		}
	}
	return name
}

// GenerateRandomString : Generates a random string of 12 characters for use in temporary filenames
func (command *Command) GenerateRandomString() string {
	const charset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 12)
	var seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// WriteScript : Convert script content provided in the JointJS element to file on disk for processing
func (command *Command) writeScript() (string, error) {
	if command.Language == "dockerfile" {
		return "", nil
	}
	var name string = command.GenerateRandomString()
	name = fmt.Sprintf("/tmp/%s-%s", command.Name, name)
	var (
		content string
		script  []byte
		err     error
	)
	if script, err = base64.StdEncoding.DecodeString(command.ScriptContent); err != nil {
		return "", err
	}
	content = string(script)

	file, err := os.Create(name)
	if err != nil {
		return "", fmt.Errorf("Failed to create temporary file for %s. %s", name, err)
	}
	defer file.Close()
	if _, err := file.WriteString(content); err != nil {
		return "", fmt.Errorf("Failed to write script contents for %s. Error was: %s", name, err)
	}
	file.Sync()
	return name, nil
}

// Execute : Execute the current command inside the container
//
// This is the main workhorse function for the application, taking all
// User configured and temporary variables, mapping them into the environment
// and process arguments then executing the command, terminating as necesssary
// after the configured timeout, and capturing the output for later storage and
// retrieval
func (command *Command) Execute(directory string, subdir string, filename string, event string) int {
	log.Info("Using environment:")
	var environment []string = append(os.Environ(), command.Environment...)

	for _, value := range environment {
		log.Info("    - ", value)
	}

	var (
		name string
		err  error
	)
	command.ProcessArgs = make([]string, 0)
	if command.ScriptContent != "" {
		name, err = command.writeScript()
		if err != nil {
			log.Error(err)
			return 1
		}
	}
	if name != "" {
		command.ProcessArgs = append(command.ProcessArgs, name)
	}

	// For certain commands, there needs to be an element of control over
	// how files are separated
	for _, item := range strings.Split(command.Args, " ") {
		log.Info("Appending argument ", item)
		switch item {
		case "--tiyo-csi":
			command.FileSeperator = ","
			break
		case "--tiyo-cssi":
			command.FileSeperator = ", "
			break
		case "--tiyo-scssi":
			command.FileSeperator = " , "
			break
		case "--tiyo-flag-f":
			command.FileSeperator = "-f"
			break
		default:
			command.ProcessArgs = append(command.ProcessArgs, item)
			break
		}
	}

	info, _ := os.Stat(directory)
	if info != nil {
		var filedir string = command.collectFiles(directory, filename)
		directory = filepath.Join(directory, subdir, filedir)
		if _, err := os.Stat(directory); err != nil {
			_ = os.Mkdir(directory, 0755)
		}
		log.Info("Setting working directory to ", directory)
		os.Chdir(directory)
	}

	log.Info("Triggering command ", command.Command, " ", command.ProcessArgs)
	cmd := exec.Command(command.Command, command.ProcessArgs...)
	cmd.Env = environment

	// dont collect logs on forever run
	if command.Timeout != -1 {
		cmd.Stdout = io.MultiWriter(os.Stdout, &command.Stdout)
		cmd.Stderr = io.MultiWriter(os.Stderr, &command.Stderr)
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	command.StartTime = time.Now().UnixNano()

	done := make(chan error)
	if err := cmd.Start(); err != nil {
		command.EndTime = time.Now().UnixNano()
		log.Error("Failed to start command `", command.Command, " ", command.Args, "`")
		log.Error("error was: ", err)
		return 1
	}

	if command.Timeout == -1 {
		return command.ExecuteForever(cmd, done)
	}

	return command.ExecuteWithTimeout(cmd, done)
}

// Globs the input directory and appends each file as an argument to the command
//
// Uses command.FileSeperator to append the filenames
// If more than 1 file is found, returns the filename as an additional directory
// otherwise returns an empty string
func (command *Command) collectFiles(directory string, filename string) string {
	if command.Timeout == -1 {
		log.Info("Not collecting files for forever run")
		return ""
	}

	var glob string = filepath.Join(directory, "*"+filename+"*")
	files, err := filepath.Glob(glob)
	if err != nil {
		log.Error(err)
		return ""
	}
	log.Info("Found ", len(files), " for fileglob ", glob)
	var dirReturn = ""
	if len(files) > 1 {
		dirReturn = filename
	}

	var csifiles string = ""
	for index, file := range files {
		if command.FileSeperator == "," {
			csifiles = strings.TrimLeft(strings.Join([]string{csifiles, file}, ","), ",")
			continue
		} else if command.FileSeperator == ", " {
			if index < len(files)-1 {
				file = file + ","
			}
		} else if command.FileSeperator == " , " {
			if index < len(files)-1 {
				file = file + " ,"
			}
		} else if command.FileSeperator == "-f" {
			command.ProcessArgs = append(command.ProcessArgs, "-f")
		}
		command.ProcessArgs = append(command.ProcessArgs, file)
	}
	if csifiles != "" {
		log.Info("Got Comma separated index of files ", csifiles)
		command.ProcessArgs = append(command.ProcessArgs, strings.Trim(csifiles, ","))
	}
	return dirReturn
}

// ExecuteForever : Executes the given command in a "forever" loop
//
// Note:
// This command does not copy any logs - presuming they will grow exponentially
// large over time. Check STDERR/STDOUT on the pod for the logs garnered by it.
func (command *Command) ExecuteForever(cmd *exec.Cmd, done chan error) int {
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		if exitError, ok := err.(*exec.ExitError); ok {
			command.EndTime = time.Now().UnixNano()
			return exitError.ExitCode()
		}
	}

	return 0
}

// ExecuteWithTimeout : Runs the given command with a timeout
//
// By default, this timeout is set to 15 minutes and this value is used if 0 is provided.
// Longer timeouts can be set or set to -1 to run forever.
func (command *Command) ExecuteWithTimeout(cmd *exec.Cmd, done chan error) int {
	go func() {
		done <- cmd.Wait()
	}()

	var exitCode int = 1
	select {
	case err := <-done:
		if exitError, ok := err.(*exec.ExitError); ok {
			command.EndTime = time.Now().UnixNano()
			exitCode = exitError.ExitCode()
		}
		break
	case <-time.After(time.Duration(command.Timeout) * time.Second):
		log.Error("Command ", command.Name, " exited due to timeout - ", command.Timeout, " seconds exceeded")
		break
	}
	command.recreateTmp()
	return exitCode
}

// recreateTmp : Deletes and recreates the temporary directory inside the container
func (command *Command) recreateTmp() {
	err := os.RemoveAll("/tmp")
	if err == nil {
		os.Mkdir("/tmp", 0775)
	}
}
