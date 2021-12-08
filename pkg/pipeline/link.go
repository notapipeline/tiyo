// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

// Link : Common structure between element link types
type Link struct {

	// JointJS ID of the current link
	ID string

	// Type of link being drawn
	Type string

	// JointJS Source element ID of the link
	Source string

	// JointJS Destination element ID of the link
	Target string
}

// LinkInterface : Common methods to read data from a link type
type LinkInterface interface {

	// GetType : Get the type of link
	GetType() string

	// GetLink : Get the common link details
	GetLink() Link
}

// NewLink : Create a new Link object
func NewLink(cell map[string]interface{}) LinkInterface {
	link := GetLink(cell)
	if link.Type == "tcp" || link.Type == "udp" {
		return NewPortLink(cell)
	}
	return NewPathLink(cell)
}

// GetLink : Unpack a link out of map[string]interface
func GetLink(cell map[string]interface{}) Link {
	link := Link{
		ID:     "",
		Type:   "",
		Source: "",
		Target: "",
	}

	if cell["id"] != nil {
		link.ID = cell["id"].(string)
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
