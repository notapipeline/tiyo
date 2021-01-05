package pipeline

import (
	"encoding/base64"
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
