package pipeline

type Link struct {
	Id     string
	Type   string
	Source string
	Target string
}

type LinkInterface interface {
	GetType() string
	GetLink() Link
}

func NewLink(cell map[string]interface{}) LinkInterface {
	link := GetLink(cell)
	if link.Type == "tcp" || link.Type == "udp" {
		return NewPortLink(cell)
	}
	return NewPathLink(cell)
}

func GetLink(cell map[string]interface{}) Link {
	link := Link{
		Id:     "",
		Type:   "",
		Source: "",
		Target: "",
	}

	if cell["id"] != nil {
		link.Id = cell["id"].(string)
	}

	attrib := cell["attributes"].(map[string]interface{})
	source := cell["source"].(map[string]interface{})
	target := cell["target"].(map[string]interface{})

	if attrib["type"] != nil {
		link.Type = attrib["type"].(string)
	}

	if source["id"] != nil {
		link.Source = source["id"].(string)
	}

	if target["id"] != nil {
		link.Target = target["id"].(string)
	}
	return link
}
