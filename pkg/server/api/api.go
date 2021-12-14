// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package api

// Primary API service for Assemble server

import (
	"regexp"
	"time"

	"github.com/boltdb/bolt"
	"github.com/notapipeline/tiyo/pkg/config"
)

// Store the list of containers globally to prevent it
// being re-downloaded each time the list is requested
var (
	containers []string

	// This little monsterous beast is because deployments use a random code in their names
	containerNameMatcher          = regexp.MustCompile(`((?P<deployment>.+)-(([0-9a-f]{0,10}-[0-9a-z]{0,5}))|(?P<container>.+)-([0-9a-zA-Z]+))$`)
	names                []string = containerNameMatcher.SubexpNames()
)

// NewResult : Create a new result item
func NewResult() *Result {
	result := Result{}
	return &result
}

// API : Assemble server api object
type API struct {

	// The bolt database held by this installation
	Db *bolt.DB

	// Server Configuration
	Config *config.Config

	// A map of queue sizes
	QueueSize map[string]int

	// The lock table for the queues
	queueLock *Lock
}

// NewAPI : Create a new API instance
func NewAPI(dbName string, c *config.Config) (*API, error) {
	api := API{
		Config: c,
	}

	var err error
	api.Db, err = bolt.Open(dbName, 0600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}
	api.QueueSize = make(map[string]int)
	lock := Lock{
		locks: make([]string, 0),
	}
	api.queueLock = &lock
	return &api, nil
}
