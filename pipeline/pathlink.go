package pipeline

// Socket / Path connections
type PathLink struct {
	Link
	Path  string
	Watch bool
}

func (path *PathLink) GetType() string {
	return path.Link.Type
}

func (path *PathLink) GetLink() Link {
	return path.Link
}

func NewPathLink(cell map[string]interface{}) *PathLink {
	link := GetLink(cell)
	path := PathLink{
		Link{
			Id:     link.Id,
			Type:   link.Type,
			Source: link.Source,
			Target: link.Target,
		},
		"", false,
	}
	attrib := cell["attributes"].(map[string]interface{})

	if attrib["path"] != nil {
		path.Path = attrib["path"].(string)
	}

	if attrib["watch"] != nil {
		path.Watch = attrib["watch"].(bool)
	}

	return &path
}
