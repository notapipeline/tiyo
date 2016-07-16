package server

import (
	"fmt"
	"html/template"
)

func LoadTemplates(paths ...string) *template.Template {
	var err error
	var tpl *template.Template
	var path string
	var data []byte
	for _, path = range paths {
		data, err = Asset("server/assets/templates/" + path)
		if err != nil {
			fmt.Println(err)
		}
		var tmp *template.Template
		if tpl == nil {
			tpl = template.New(path)
		}
		if path == tpl.Name() {
			tmp = tpl
		} else {
			tmp = tpl.New(path)
		}
		_, err = tmp.Delims("[[", "]]").Parse(string(data))
		if err != nil {
			fmt.Println(err)
		}
	}
	return tpl
}
