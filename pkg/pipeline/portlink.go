// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

// PortLink : TCP / UDP connections between elements
type PortLink struct {
	Link

	// Source port for connections
	SourcePort int

	// Destination port on the service
	DestPort int

	// Address to connect to
	Address string
}

// GetType : Gets the type of PortLink (TCP/UDP)
func (port *PortLink) GetType() string {
	return port.Link.Type
}

// GetLink : Get details about this link
func (port PortLink) GetLink() Link {
	return port.Link
}

// NewPortLink : Create a new PortLink object
func NewPortLink(cell map[string]interface{}) *PortLink {
	link := GetLink(cell)
	port := PortLink{
		Link{
			ID:     link.ID,
			Type:   link.Type,
			Source: link.Source,
			Target: link.Target,
		},
		0, 0, "",
	}
	attrib := cell["attributes"].(map[string]interface{})

	if attrib["source"] != nil {
		port.SourcePort = int(attrib["source"].(float64))
	}

	if attrib["dest"] != nil {
		port.DestPort = int(attrib["dest"].(float64))
	}

	if attrib["address"] != nil {
		port.Address = attrib["address"].(string)
	}
	return &port
}
