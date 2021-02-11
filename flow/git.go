// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package flow

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/notapipeline/tiyo/config"
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

	// Flow configuration object
	config *config.Config

	// The loaded git repository
	repository *git.Repository
}

// NewGitRepo : Create a git repository object
func NewGitRepo(config *config.Config) *GitRepo {
	repo := GitRepo{
		config: config,
	}
	return &repo
}

// Clone : Clone out a git repository from source
func (gitRepo *GitRepo) Clone() error {
	log.Info("Cloning ", gitRepo.RepoURL)
	var basename string = strings.TrimSuffix(filepath.Base(gitRepo.RepoURL), ".git")
	var path = gitRepo.config.SequenceBaseDir + "/source/" + basename
	var err error

	if gitRepo.repository, err = git.PlainClone(path, false, &git.CloneOptions{
		Auth: &http.BasicAuth{
			Username: gitRepo.Username,
			Password: gitRepo.Password,
		},
		URL:      gitRepo.RepoURL,
		Progress: os.Stdout,
	}); err != nil {
		return err
	}
	return nil
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
