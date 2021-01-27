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
	Id            string `json:"id"`
	Parent        string `json:"parent"`
	Name          string `json:"name"`
	Command       string `json:"command"`
	Args          string `json:"args"`
	Version       string `json:"version"`
	Language      string `json:"element"`
	Script        bool   `json:"script"`
	ScriptContent string `json:"scriptcontent"`
	Custom        bool   `json:"custom"`
	Timeout       int    `json:"timeout"`
	UseExisting   bool   `json:"existing"`
	ExposePort    int    `json:"exposeport"`
	IsUdp         bool   `json:"isudp"`
	StartTime     int64  `json:""`
	EndTime       int64  `json:""`

	Image         string
	Tag           string
	Stdout        bytes.Buffer
	Stderr        bytes.Buffer
	ProcessId     int
	ProcessArgs   []string
	FileSeperator string
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
		Timeout:       DEFAULT_TIMEOUT,
		UseExisting:   false,
		ExposePort:    -1,
		IsUdp:         false,
		StartTime:     0,
		EndTime:       0,
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
			command.Timeout = DEFAULT_TIMEOUT
		}
		if command.Timeout != -1 {
			command.Timeout = command.Timeout * 60
		}
	} else {
		command.Timeout = DEFAULT_TIMEOUT * 60
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

func (command *Command) writeScript() (string, error) {
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

func (command *Command) Execute(directory string, subdir string, filename string, event string) int {
	log.Info("Using environment:")
	for _, value := range os.Environ() {
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
		command.collectFiles(directory, filename)
		directory = filepath.Join(directory, subdir)
		if _, err := os.Stat(directory); err != nil {
			_ = os.Mkdir(directory, 0755)
		}
		log.Info("Setting working directory to ", directory)
		os.Chdir(directory)
	}

	log.Info("Triggering command ", command.Command, " ", command.ProcessArgs)
	cmd := exec.Command(command.Command, command.ProcessArgs...)
	cmd.Env = os.Environ()

	// dont collect logs on forever run
	if command.Timeout != -1 {
		cmd.Stdout = io.MultiWriter(os.Stdout, &command.Stdout)
		cmd.Stderr = io.MultiWriter(os.Stderr, &command.Stderr)
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

func (command *Command) collectFiles(directory string, filename string) {
	if command.Timeout == -1 {
		log.Info("Not collecting files for forever run")
		return
	}

	var glob string = filepath.Join(directory, "*"+filename+"*")
	files, err := filepath.Glob(glob)
	if err != nil {
		log.Error(err)
		return
	}
	log.Info("Found ", len(files), " for fileglob ", glob)

	var csifiles string = ""
	for index, file := range files {
		if command.FileSeperator == "," {
			csifiles = csifiles + ","
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
		command.ProcessArgs = append(command.ProcessArgs, strings.Trim(csifiles, ","))
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
	case <-time.After(time.Duration(command.Timeout) * time.Second):
		log.Error("Command ", command.Name, " exited due to timeout - ", command.Timeout, " seconds exceeded")
		return 1
	}

	return 0
}
