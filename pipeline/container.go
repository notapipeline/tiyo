package pipeline

type Container struct {
	Id       string
	Name     string
	Scale    int32
	Children []string
	SetType  string

	Pipeline *Pipeline
}

func NewContainer(pipeline *Pipeline, cell map[string]interface{}) *Container {
	container := Container{
		Pipeline: pipeline,
	}

	if cell["id"] != nil {
		container.Id = cell["id"].(string)
	}

	if cell["name"] != nil {
		container.Name = cell["name"].(string)
	}

	if cell["settype"] != nil {
		container.SetType = cell["settype"].(string)
	}

	if cell["scale"] != nil {
		container.Scale = int32(cell["scale"].(float64))
	}

	if cell["embeds"] != nil {
		container.Children = make([]string, 0)
		for _, item := range cell["embeds"].([]interface{}) {
			container.Children = append(container.Children, item.(string))
		}
	}

	return &container
}

func (container *Container) GetChildren() []*Command {
	commands := make([]*Command, 0)
	for _, command := range container.Pipeline.Commands {
		if command.Parent == container.Id {
			commands = append(commands, command)
		}
	}
	return commands
}
