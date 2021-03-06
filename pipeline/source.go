// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

// Defines a source data structure type
// At present, this maps to one of:
//   - File
//   - Directory
//   - Stream

// Source : A source data structure
type Source struct {

	// The JointJS ID of this element
	ID string `json:"id"`

	// The name of this element in the pipeline
	Name string `json:"name"`

	// The source type of this element
	Type string `json:"sourcetype"`
}

// NewSource : create a new source object
func NewSource(cell map[string]interface{}) *Source {
	source := Source{}
	if cell["id"] != nil {
		source.ID = cell["id"].(string)
	}

	if cell["name"] != nil {
		source.Name = cell["name"].(string)
	}

	if cell["sourcetype"] != nil {
		source.Type = cell["sourcetype"].(string)
	}

	return &source
}
