// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	log "github.com/sirupsen/logrus"
)

// GitRepo : Handle git operations for the pipeline
type GitRepo struct {

	// The repository URL
	RepoURL string `json:"repo"`

	// The branch or commit to checkout on to
	Branch string `json:"branch"`

	// The Username to checkout with
	Username string `json:"username"`

	// The password or token to use for checkout
	Password string `json:"password"`

	// A script to act as the entrypoint inside the container
	Entrypoint string `json:"entrypoint"`

	// The loaded git repository
	repository *git.Repository
}

// NewGitRepo : Create a git repository object
func NewGitRepo() *GitRepo {
	repo := GitRepo{}
	return &repo
}

// Set up the GitRepo from form input
func (gitRepo *GitRepo) Configure(values map[string]interface{}) {
	log.Debug(values)
	if values["repo"] != nil {
		gitRepo.RepoURL = values["repo"].(string)
	}
	if values["branch"] != nil {
		gitRepo.Branch = values["branch"].(string)
	}
	if values["username"] != nil {
		gitRepo.Username = values["username"].(string)
	}
	if values["password"] != nil {
		gitRepo.Password = values["password"].(string)
	}
	if values["entrypoint"] != nil {
		gitRepo.Entrypoint = values["entrypoint"].(string)
	}
}

func (gitRepo *GitRepo) HasKey(where map[string]string, key string) bool {
	if _, ok := where[key]; ok {
		return true
	}
	return false
}

func (gitRepo *GitRepo) Clone(destination string, options map[string]string) {
	if gitRepo.HasKey(options, "key") && gitRepo.HasKey(options, "password") {
		gitRepo.ssh(destination, options["key"], options["password"])
	} else if gitRepo.HasKey(options, "password") {
		gitRepo.basic(destination, options["password"])
	}
}

// Clone : Clone out a git repository from source
func (gitRepo *GitRepo) basic(destination string, password string) error {
	log.Info("Cloning ", gitRepo.RepoURL)
	var basename string = strings.TrimSuffix(filepath.Base(gitRepo.RepoURL), ".git")
	var path = destination + "/" + basename
	var err error

	if gitRepo.repository, err = git.PlainClone(path, false, &git.CloneOptions{
		Auth: &http.BasicAuth{
			Username: gitRepo.Username,
			Password: password,
		},
		URL:               gitRepo.RepoURL,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		Progress:          os.Stdout,
	}); err != nil {
		return err
	}
	return nil
}

func (gitRepo *GitRepo) ssh(destination string, key string, password string) {
	// Not implemented
}

// Checkout : Checks out a given branch or revision
func (gitRepo *GitRepo) Checkout() error {
	var worktree *git.Worktree
	var err error
	if worktree, err = gitRepo.repository.Worktree(); err != nil {
		return err
	}

	var hash *plumbing.Hash
	if hash, err = gitRepo.repository.ResolveRevision(plumbing.Revision(gitRepo.Branch)); err != nil {
		return err
	}

	if err = worktree.Checkout(&git.CheckoutOptions{
		Hash: *hash,
	}); err != nil {
		return err
	}
	return nil
}
