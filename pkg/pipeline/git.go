// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
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

	// options used for clone
	cloneoptions git.CloneOptions

	// options used for pull
	pulloptions git.PullOptions

	// keyfile name
	keyfile string
}

// NewGitRepo : Create a git repository object
func NewGitRepo() *GitRepo {
	repo := GitRepo{}
	return &repo
}

// Set up the GitRepo from form input
func (gitRepo *GitRepo) Configure(values map[string]interface{}) {
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

// HasKey : Check if a given map has a particular key
func (gitRepo *GitRepo) HasKey(where map[string]string, key string) bool {
	if _, ok := where[key]; ok {
		return true
	}
	return false
}

// Clone : Clone out a given repo
func (gitRepo *GitRepo) Clone(destination string, options map[string]string) error {
	log.Info("Cloning ", gitRepo.RepoURL)
	var basename string = strings.TrimSuffix(filepath.Base(gitRepo.RepoURL), ".git")
	destination = destination + "/" + basename
	gitRepo.keyfile = destination + ".key"
	if err := gitRepo.buildOptions(options); err != nil {
		return err
	}

	if _, err := os.Stat(destination); os.IsExist(err) {
		if gitRepo.repository, err = git.PlainOpen(destination); err != nil {
			return err
		}
	} else {
		var err error
		if gitRepo.repository, err = git.PlainClone(destination, false, &gitRepo.cloneoptions); err != nil {
			return err
		}
	}
	return nil
}

func (gitRepo *GitRepo) buildOptions(options map[string]string) error {
	gitRepo.cloneoptions = git.CloneOptions{
		URL:               gitRepo.RepoURL,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		Progress:          os.Stdout,
	}

	gitRepo.pulloptions = git.PullOptions{
		RemoteName:        "origin",
		SingleBranch:      true,
		Depth:             1,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		Progress:          os.Stdout,
		Force:             false,
	}

	if gitRepo.HasKey(options, "key") && gitRepo.HasKey(options, "password") {
		if err := gitRepo.sshAuth(options["key"], options["password"]); err != nil {
			return err
		}
	} else if gitRepo.HasKey(options, "password") {
		gitRepo.basicAuth(options["password"])
	}

	return nil
}

// sshAuth : Generate SSH authentication
func (gitRepo *GitRepo) sshAuth(key string, password string) error {
	if _, err := os.Stat(key); err != nil {
		file, err := os.Create(gitRepo.keyfile)
		if err != nil {
			return fmt.Errorf("Failed to Create SSH key for %s. %s", gitRepo.RepoURL, err)
		}
		defer file.Close()
		if _, err := file.WriteString(key); err != nil {
			return fmt.Errorf("Failed to write ssh key contents for %s. Error was: %s", gitRepo.RepoURL, err)
		}

		return fmt.Errorf("read file %s failed %s", key, err.Error())
	} else {
		gitRepo.keyfile = key
	}

	// Clone the given repository to the given directory
	publicKeys, err := ssh.NewPublicKeysFromFile("git", key, password)
	if err != nil {
		return fmt.Errorf("generate publickeys failed: %s", err.Error())
	}
	gitRepo.cloneoptions.Auth = publicKeys
	gitRepo.pulloptions.Auth = publicKeys
	return nil
}

// Clone : Set up basic auth options
func (gitRepo *GitRepo) basicAuth(password string) {
	gitRepo.cloneoptions.Auth = &http.BasicAuth{
		Username: gitRepo.Username,
		Password: password,
	}
	gitRepo.pulloptions.Auth = &http.BasicAuth{
		Username: gitRepo.Username,
		Password: password,
	}
}

// Checkout : Checks out a given branch or revision
func (gitRepo *GitRepo) Checkout() error {
	var worktree *git.Worktree
	var err error
	if worktree, err = gitRepo.repository.Worktree(); err != nil {
		return err
	}

	var hash *plumbing.Hash
	if hash, err = gitRepo.repository.ResolveRevision(plumbing.Revision("origin/" + gitRepo.Branch)); err != nil {
		return err
	}

	if err = worktree.Checkout(&git.CheckoutOptions{
		Hash: *hash,
	}); err != nil {
		return err
	}

	if err := worktree.Pull(&gitRepo.pulloptions); err != nil {
		return err
	}
	return nil
}
