// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

// A PathLink specifies how to connect files, directories and sockets between
// components in the pipeline.

// PathLink : Socket / Path connections
type PathLink struct {

	// Common Link details
	Link

	// The path this link will listen against
	// If link type is Path, this is mutually exclusive with Pattern
	Path string

	// Regex pattern to match when reading paths
	// ignored if link type is socket
	Pattern string

	// If link type is path sets up inotify watchers for the path
	Watch bool
}

// GetType : Get the type of link
func (path *PathLink) GetType() string {
	return path.Link.Type
}

// GetLink : Get common link details
func (path *PathLink) GetLink() Link {
	return path.Link
}

// NewPathLink : Convert a map[string]interface into a PathLink type
func NewPathLink(cell map[string]interface{}) *PathLink {
	link := GetLink(cell)
	path := PathLink{
		Link{
			ID:     link.ID,
			Type:   link.Type,
			Source: link.Source,
			Target: link.Target,
		},
		"", "", false,
	}
	attrib := cell["attributes"].(map[string]interface{})

	if attrib["path"] != nil {
		path.Path = attrib["path"].(string)
	}

	if attrib["pattern"] != nil {
		path.Pattern = attrib["pattern"].(string)
	}

	if attrib["watch"] != nil {
		path.Watch = attrib["watch"].(bool)
	}

	return &path
}
