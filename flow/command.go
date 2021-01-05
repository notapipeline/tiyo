package flow

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"time"
)

type SimpleCommand struct {
	Id            string `json:"id"`
	Name          string `json:"name"`
	Command       string `json:"command"`
	Timeout       int    `json:"timeout"`
	Args          string `json:"arguments"`
	ScriptContent string `json:"script"`
}

type Command struct {
	SimpleCommand
	Stdout    bytes.Buffer
	Stderr    bytes.Buffer
	ProcessId int

	StartTime int64
	EndTime   int64
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
	if command.ScriptContent != "" {
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
