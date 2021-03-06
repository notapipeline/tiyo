// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"os"

	"github.com/notapipeline/tiyo/command"
)

func main() {
	os.Exit(command.Run(os.Args[1:]))
}
