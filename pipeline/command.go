package pipeline

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"time"
)

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

	Stdout    bytes.Buffer
	Stderr    bytes.Buffer
	ProcessId int

	StartTime int64
	EndTime   int64
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
	}

	if cell["id"] != nil {
		command.Id = cell["id"].(string)
	}

	if cell["parent"] != nil {
		command.Parent = cell["parent"].(string)
	}

	if cell["name"] != nil {
		command.Name = cell["name"].(string)
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

	if cell["lang"] != nil {
		command.Language = cell["lang"].(string)
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

	if !command.Custom {
		if asTag {
			return command.Name + "-tiyo:" + command.Version
		}
		return command.Name + ":" + command.Version
	}
	if asTag {
		return container + "-tiyo:" + command.Version
	}
	return container + ":" + command.Version
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

func (command *Command) Execute() int {
	if command.Script && command.ScriptContent != "" {
		name, err := command.WriteScript()
		if err != nil {
			fmt.Printf("%s", err)
		}
		command.Args += fmt.Sprintf(" %s", name)
	}
	cmd := exec.Command(command.Command, command.Args)
	cmd.Stdout = io.MultiWriter(os.Stdout, &command.Stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &command.Stderr)
	command.StartTime = time.Now().UnixNano()
	if err := cmd.Start(); err != nil {
		command.EndTime = time.Now().UnixNano()
		fmt.Printf("Failed to start command '%s %s'\n", command.Command, command.Args)
		fmt.Printf("error was: %+v\n", err)
		return 1
	}

	if err := cmd.Wait(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			command.EndTime = time.Now().UnixNano()
			return exitError.ExitCode()
		}
	}
	command.EndTime = time.Now().UnixNano()
	return 0
}
