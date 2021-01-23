package pipeline

type Source struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"sourcetype"`
}

func NewSource(cell map[string]interface{}) *Source {
	source := Source{}
	if cell["id"] != nil {
		source.Id = cell["id"].(string)
	}

	if cell["name"] != nil {
		source.Name = cell["name"].(string)
	}

	if cell["sourcetype"] != nil {
		source.Type = cell["sourcetype"].(string)
	}

	return &source
}
