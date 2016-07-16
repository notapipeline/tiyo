package pipeline

// TCP / UDP connections
type PortLink struct {
	Link
	SourcePort int
	DestPort   int
	Address    string
}

func (port *PortLink) GetType() string {
	return port.Link.Type
}

func (port PortLink) GetLink() Link {
	return port.Link
}

func NewPortLink(cell map[string]interface{}) *PortLink {
	link := GetLink(cell)
	port := PortLink{
		Link{
			Id:     link.Id,
			Type:   link.Type,
			Source: link.Source,
			Target: link.Target,
		},
		0, 0, "",
	}
	attrib := cell["attributes"].(map[string]interface{})

	if attrib["source"] != nil {
		port.SourcePort = attrib["source"].(int)
	}

	if attrib["dest"] != nil {
		port.DestPort = attrib["destintation"].(int)
	}

	if attrib["address"] != nil {
		port.Address = attrib["address"].(string)
	}
	return &port
}
