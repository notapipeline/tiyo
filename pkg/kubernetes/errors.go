package kubernetes

import "fmt"

type noResourceListsFound struct{}

func (noResourceListsFound) Error() string {
	return "No resource lists found on the api"
}

var NoResourceListsFound error = noResourceListsFound{}

type noVerbs struct{}

func (noVerbs) Error() string {
	return "No verbs found"
}

var NoVerbs error = noVerbs{}

type InvalidV3Schema struct {
	rg string
	rv string
}

func NewInvalidV3Schema(rg, rv string) *InvalidV3Schema {
	e := InvalidV3Schema{
		rg: rg,
		rv: rv,
	}
	return &e
}

func (e InvalidV3Schema) Error() string {
	return fmt.Sprintf("Invalid open api version 3 schema for %s/%s", e.rg, e.rg)
}

type containsErrors struct{}

func (containsErrors) Error() string {
	return "results list contains errors"
}

var ContainsErrors error = containsErrors{}
