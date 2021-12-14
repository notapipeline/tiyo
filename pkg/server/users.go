// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/google/uuid"
	"github.com/notapipeline/tiyo/pkg/server/api"
)

const (
	USERS_T       = "users"
	USERGROUPS_T  = "groups"
	PASSW_T       = "passwords"
	TOTP_T        = "totp"
	GROUP_T       = "group"
	GROUP_PERMS_T = "groupperms"
	ACL_T         = "acl"
	PERM_T        = "permissions"
)

type Lockable struct {
	sync.Mutex
	Db *bolt.DB
}

type User struct {
	ID       string
	Email    string
	Password string
	Groups   []Group
	TotpKey  string
}

type Group struct {
	ID          string
	Name        string
	Permissions []Permission
}

type Permission struct {
	ID         string
	Permission string
}

var tables []string = []string{
	USERS_T, USERGROUPS_T, PASSW_T, TOTP_T, GROUP_T, ACL_T, PERM_T,
}

func (s *Server) CreateTables() error {
	s.users.Lock()
	for _, table := range tables {
		if err := s.users.Db.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte(table))
			if err != nil {
				return fmt.Errorf("creating users table %s: %s", table, err)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	s.users.Unlock()
	return nil
}

func (s *Server) AddUser(user *User) error {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	u := s.encryptUser(user)

	if err := s.users.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(USERS_T))
		err := b.Put([]byte(user.ID), []byte(u.Email))
		if err != nil {
			return fmt.Errorf("Failed to add or update user: %s", err)
		}

		b = tx.Bucket([]byte(PASSW_T))
		err = b.Put([]byte(u.ID), []byte(u.Password))
		if err != nil {
			// error HMAC only, do not error user values
			return fmt.Errorf("AddUser password failed: %s", err)
		}

		b = tx.Bucket([]byte(TOTP_T))
		err = b.Put([]byte(u.ID), []byte(u.TotpKey))
		if err != nil {
			// error HMAC only, do not error user values
			return fmt.Errorf("AddUser totp failed: %s", err)
		}

		var groups []string = make([]string, 0)
		for _, group := range u.Groups {
			groups = append(groups, group.ID)
		}
		groupsString := strings.Join(groups, ",")
		b = tx.Bucket([]byte(USERGROUPS_T))
		err = b.Put([]byte(u.ID), []byte(groupsString))
		if err != nil {
			return fmt.Errorf("AddUser failed adding groups: %s", err)
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *Server) FindUser(email string) (*User, error) {
	var user User = User{
		Email: email,
	}
	val, _ := api.EncryptData([]byte(email), s.config.GetPassphrase("assemble"))
	e := base64.StdEncoding.EncodeToString(val)

	if err := s.users.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(USERS_T))
		c := b.Cursor()
		var id []byte
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if 0 == bytes.Compare(v, []byte(e)) {
				id = k
			}
		}
		user.ID = string(id)

		b = tx.Bucket([]byte(PASSW_T))
		password := b.Get(id)
		user.Password = fmt.Sprintf("%s", password)

		b = tx.Bucket([]byte(TOTP_T))
		totp := b.Get(id)
		inter, _ := base64.StdEncoding.DecodeString(string(totp))
		totp, _ = api.DecryptData([]byte(inter), s.config.GetPassphrase("assemble"))
		user.TotpKey = string(totp)

		var groups []Group = make([]Group, 0)
		b = tx.Bucket([]byte(USERGROUPS_T))
		g := strings.Split(string(b.Get(id)), ",")
		for _, n := range g {
			group, _ := s.FindGroup(n)
			groups = append(groups, *group)
		}

		user.Groups = groups

		return nil
	}); err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Server) DeleteUser(user *User) error {
	return nil
}

func (s *Server) AddGroup(group *Group) error {
	if group.ID == "" {
		group.ID = uuid.New().String()
	}
	if err := s.users.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(GROUP_T))
		err := b.Put([]byte(group.ID), []byte(group.Name))
		if err != nil {
			return fmt.Errorf("Failed to add or update group: %s", err)
		}

		var permIds []string = make([]string, 0)
		for _, p := range group.Permissions {
			permIds = append(permIds, p.ID)
		}
		permString := strings.Join(permIds, ",")
		b = tx.Bucket([]byte(GROUP_PERMS_T))
		err = b.Put([]byte(group.ID), []byte(permString))
		if err != nil {
			return fmt.Errorf("Failed to add or update group permissions: %s", err)
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *Server) FindGroup(name string) (*Group, error) {
	var group Group = Group{
		Name: name,
	}

	if err := s.users.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(GROUP_T))
		c := b.Cursor()
		var id []byte
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if 0 == bytes.Compare(v, []byte(name)) {
				id = k
			}
		}
		group.ID = string(id)
		b = tx.Bucket([]byte(GROUP_PERMS_T))
		perms := strings.Split(string(b.Get(id)), ",")

		var permissions []Permission = make([]Permission, 0)
		b = tx.Bucket([]byte(PERM_T))
		for _, p := range perms {
			permission := b.Get([]byte(p))
			permissions = append(permissions, Permission{
				ID:         string(permission),
				Permission: string(id),
			})
		}
		group.Permissions = permissions

		return nil
	}); err != nil {
		return nil, err
	}

	return &group, nil
}

func (s *Server) DeleteGroup(name string) error {
	return nil
}

func (s *Server) AddPermission(name string) error {
	var id = uuid.New().String()
	if err := s.users.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PERM_T))
		err := b.Put([]byte(id), []byte(name))
		if err != nil {
			return fmt.Errorf("Failed to add or update permission: %s", err)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *Server) encryptUser(user *User) User {
	u := User{}

	// MUST BE HASHED
	val, _ := api.EncryptData([]byte(user.Email), s.config.GetPassphrase("assemble"))
	u.Email = base64.StdEncoding.EncodeToString(val)

	// MUST BE HASHED
	val, _ = api.EncryptData([]byte(user.Password), s.config.GetPassphrase("assemble"))
	u.Password = base64.StdEncoding.EncodeToString(val)

	// MUST BE REVERSABLE
	val, _ = api.EncryptData([]byte(user.TotpKey), s.config.GetPassphrase("assemble"))
	u.TotpKey = base64.StdEncoding.EncodeToString(val)

	return u
}
