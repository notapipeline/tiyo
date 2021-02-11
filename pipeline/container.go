// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

// The container structure defines the outer structure of a set type
// This will be one of
// - DaemonSet
// - Deployment
// - StatefulSet
//
// This file should not be confused with docker containers which are defined
// inside the command structure

// Container :_Overall structure of a set container
type Container struct {

	// The JointJS ID of this container element
	ID string

	// The name of the container element
	Name string

	// How many pods to build
	Scale int32

	// A list of all child IDs this container manages
	Children []string

	// The type of container
	SetType string

	// The containers state (Building, Ready, Destroying)
	State string

	// The number of pods last seen
	LastCount int

	// The pipeline this container belongs to
	Pipeline *Pipeline
}

// NewContainer : Construct a new container instance
func NewContainer(pipeline *Pipeline, cell map[string]interface{}) *Container {
	container := Container{
		Pipeline:  pipeline,
		LastCount: 0,
	}

	if cell["id"] != nil {
		container.ID = cell["id"].(string)
	}

	if cell["name"] != nil {
		container.Name = Sanitize(cell["name"].(string), "-")
	}

	if cell["settype"] != nil {
		container.SetType = cell["settype"].(string)
	}

	if cell["scale"] != nil {
		container.Scale = int32(cell["scale"].(float64))
	}

	if cell["embeds"] != nil {
		container.Children = make([]string, 0)
		for _, item := range cell["embeds"].([]interface{}) {
			container.Children = append(container.Children, item.(string))
		}
	}

	return &container
}

// GetChildren ; Get the set of child command containers of this container
func (container *Container) GetChildren() []*Command {
	commands := make([]*Command, 0)
	for _, command := range container.Pipeline.Commands {
		if command.Parent == container.ID {
			commands = append(commands, command)
		}
	}
	return commands
}
