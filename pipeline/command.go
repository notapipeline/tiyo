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

const DEFAULT_TIMEOUT = 15

type Command struct {
	Id            string
	Parent        string
	Name          string
	Command       string
	Args          string
	Version       string
	Language      string
	Script        bool
	ScriptContent string
	Custom        bool
	Scale         int
	Timeout       int
	UseExisting   bool
	Image         string
	Tag           string
	ExposePort    int
	IsUdp         bool

	Stdout    bytes.Buffer
	Stderr    bytes.Buffer
	ProcessId int
	StartTime int64
	EndTime   int64
}

var regex *regexp.Regexp

func sanitize(str string, sep string) string {
	regex, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		log.Fatal("Failed to compile regex - ", err)
	}
	return strings.Trim(strings.ToLower(regex.ReplaceAllString(str, sep)), sep)
}

func NewCommand(cell map[string]interface{}) *Command {
	command := Command{
		Id:            "",
		Name:          "",
		Command:       "",
		Args:          "",
		Version:       "",
		Language:      "",
		Script:        false,
		ScriptContent: "",
		Custom:        false,
		Scale:         0,
		Timeout:       900,
		UseExisting:   false,
		ExposePort:    -1,
		IsUdp:         false,
	}

	if cell["id"] != nil {
		command.Id = cell["id"].(string)
	}

	if cell["parent"] != nil {
		command.Parent = cell["parent"].(string)
	}

	if cell["name"] != nil {
		command.Name = sanitize(cell["name"].(string), "-")
	}

	if cell["command"] != nil {
		command.Command = cell["command"].(string)
	}

	if cell["args"] != nil {
		command.Args = cell["args"].(string)
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

	if cell["timeout"] != nil {
		command.Timeout = int(cell["timeout"].(float64)) * 60
	}

	if cell["existing"] != nil {
		command.UseExisting = cell["existing"].(bool)
	}

	if cell["exposeport"] != nil {
		command.ExposePort = int(cell["exposeport"].(float64))
	}

	if cell["isudp"] != nil {
		command.IsUdp = cell["isudp"].(bool)
	}

	if cell["scriptcontent"] != nil {
		var script []byte
		var err error
		if script, err = base64.StdEncoding.DecodeString(cell["scriptcontent"].(string)); err != nil {
			command.ScriptContent = ""
		}
		command.ScriptContent = string(script)
	}

	if cell["custom"] != nil {
		command.Custom = cell["custom"].(bool)
	}

	return &command
}

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

func (command *Command) WriteScript() (string, error) {
	var name string = command.GenerateRandomString()
	name = fmt.Sprintf("/tmp/%s-%s", command.Name, name)

	file, err := os.Create(name)
	if err != nil {
		return "", fmt.Errorf("Failed to create temporary file for %s. %s", name, err)
	}
	defer file.Close()
	if _, err := file.WriteString(command.ScriptContent); err != nil {
		return "", fmt.Errorf("Failed to write script contents for %s. Error was: %s", name, err)
	}
	file.Sync()
	return name, nil
}

func (command *Command) Execute(directory string, filename string, event string) int {
	if command.Timeout == 0 {
		command.Timeout = DEFAULT_TIMEOUT
	}

	command.writeScript()
	command.collectFiles(directory, filename)

	cmd := exec.Command(command.Command, command.Args)
	cmd.Stdout = io.MultiWriter(os.Stdout, &command.Stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &command.Stderr)
	command.StartTime = time.Now().UnixNano()

	done := make(chan error)
	if err := cmd.Start(); err != nil {
		command.EndTime = time.Now().UnixNano()
		log.Error("Failed to start command `", command.Command, " ", command.Args, "`")
		log.Error("error was: ", err)
		return 1
	}

	if command.Timeout == -1 {
		// run forever - does not obtain dir/filename/event
		return command.ExecuteForever(cmd, done)
	}

	return command.ExecuteWithTimeout(cmd, done)
}

func (command *Command) writeScript() {
	if command.ScriptContent != "" {
		name, err := command.WriteScript()
		if err != nil {
			fmt.Printf("%s", err)
		}
		command.Args += fmt.Sprintf(" %s", name)
	}
}

func (command *Command) collectFiles(directory string, filename string) {
	files, err := filepath.Glob(filepath.Join(directory, ".*"+filename+".*"))
	if err != nil {
		// nothing to do, nothing to add
		log.Error(err)
		return
	}

	for _, f := range files {
		command.Args += " " + f
	}
}

// Executes the given command in a "forever" loop
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

// Runs the given command with a timeout
//
// By default, this timeout is set to 15 minutes and this value is used if 0 is provided.
// Longer timeouts can be set or set to -1 to run forever.
func (command *Command) ExecuteWithTimeout(cmd *exec.Cmd, done chan error) int {
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		if exitError, ok := err.(*exec.ExitError); ok {
			command.EndTime = time.Now().UnixNano()
			return exitError.ExitCode()
		}
	case <-time.After(time.Duration(command.Timeout) * time.Minute):
		log.Warn("Command ", command.Name, " exited due to timeout - ", command.Timeout, " minutes exceeded")
		return 1
	}

	return 0
}
