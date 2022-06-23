// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"html/template"
)

// LoadTemplates : Load HTML template files
func LoadTemplates(paths ...string) *template.Template {
	var err error
	var tpl *template.Template
	var path string
	var data []byte
	for _, path = range paths {
		data, err = Asset("pkg/flow/assets/templates/" + path)
		if err != nil {
			return nil
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
			return nil
		}
	}
	return tpl
}
