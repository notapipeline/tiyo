// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import "strings"

// The controller structure defines the outer structure of a set type
// This will be one of
// - DaemonSet
// - Deployment
// - StatefulSet
//
// This file should not be confused with docker controllers which are defined
// inside the command structure

// Controller :_Overall structure of a set controller
type Controller struct {

	// The JointJS ID of this controller element
	ID string

	// The name of the controller element
	Name string

	// How many pods to build
	Scale int32

	// A list of all child IDs this controller manages
	Children []string

	// The type of controller
	SourceType string

	// The controllers state (Building, Ready, Destroying)
	State string

	// The number of pods last seen
	LastCount int

	// The pipeline this controller belongs to
	Pipeline *Pipeline

	// Environment settings for the all pods under this set
	Environment []string
}

// NewController : Construct a new controller instance
func NewController(pipeline *Pipeline, cell map[string]interface{}) *Controller {
	controller := Controller{
		Pipeline:  pipeline,
		LastCount: 0,
	}
	controller.Environment = make([]string, 0)

	if cell["id"] != nil {
		controller.ID = cell["id"].(string)
	}

	if cell["name"] != nil {
		controller.Name = Sanitize(cell["name"].(string), "-")
	}

	if cell["sourcetype"] != nil {
		controller.SourceType = cell["sourcetype"].(string)
	}

	if cell["scale"] != nil {
		controller.Scale = int32(cell["scale"].(float64))
	}

	if cell["embeds"] != nil {
		controller.Children = make([]string, 0)
		for _, item := range cell["embeds"].([]interface{}) {
			controller.Children = append(controller.Children, item.(string))
		}
	}

	if cell["environment"] != nil {
		envvars := cell["environment"].([]interface{})
		for _, value := range envvars {
			controller.Environment = append(controller.Environment, value.(string))
		}
	}

	return &controller
}

// GetChildren ; Get the set of child command controllers of this controller
func (controller *Controller) GetChildren() []*Command {
	commands := make([]*Command, 0)
	for _, command := range controller.Pipeline.Commands {
		if command.Parent == controller.ID {
			commands = append(commands, command)
		}
	}
	return commands
}

// ConrollerFromServiceName : Get a container for a given kubernetes service
func (pipeline *Pipeline) ControllerFromServiceName(serviceName string) *Controller {
	for _, controller := range pipeline.Controllers {
		if strings.HasSuffix(serviceName, controller.Name) {
			return controller
		}
	}
	return nil
}

// ControllerFromCommandID : Get a container from a command id
func (pipeline *Pipeline) ControllerFromCommandID(containerID string) *Controller {
	for _, controller := range pipeline.Controllers {
		for _, element := range controller.GetChildren() {
			if element.ID == containerID {
				return controller
			}
		}
	}
	return nil
}
