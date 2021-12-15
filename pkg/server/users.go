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
	"time"

	"github.com/boltdb/bolt"
	"github.com/google/uuid"
	"github.com/notapipeline/tiyo/pkg/server/api"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

const (
	USERS_T        = "users"
	USERGROUPS_T   = "groups"
	PASSW_T        = "passwords"
	TOTP_T         = "totp"
	GROUP_T        = "group"
	GROUP_PERMS_T  = "groupperms"
	ACL_T          = "acl"
	PERM_T         = "permissions"
	MACHINE_TOKENS = "machines"
	MACHINE_HMAC   = "hmac"
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
	USERS_T, USERGROUPS_T, PASSW_T, TOTP_T, GROUP_PERMS_T, GROUP_T, PERM_T, MACHINE_TOKENS, MACHINE_HMAC,
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

	err := s.replace(user.ID, USERS_T, u.Email)
	if err != nil {
		return fmt.Errorf("Failed to add or update user: %s", err)
	}

	err = s.replace(user.ID, PASSW_T, u.Password)
	if err != nil {
		return fmt.Errorf("AddUser password failed: %s", err)
	}

	err = s.replace(user.ID, TOTP_T, u.TotpKey)
	if err != nil {
		return fmt.Errorf("AddUser totp failed: %s", err)
	}

	var groups []string = make([]string, 0)
	for _, group := range user.Groups {
		groups = append(groups, group.ID)
	}

	groupsString := strings.Join(groups, ",")
	err = s.replace(u.ID, USERGROUPS_T, groupsString)
	if err != nil {
		return fmt.Errorf("AddUser failed adding groups: %s", err)
	}

	return nil
}

func (s *Server) FindUser(email string) (*User, error) {
	var user User = User{
		Email: email,
	}

	user.ID = s.findHashedValue(email, USERS_T)
	if user.ID == "" {
		return nil, fmt.Errorf("FindUser - Failed to find user with: %s", email)
	}

	user.Password = string(s.get(user.ID, PASSW_T))

	inter := s.get(user.ID, TOTP_T)
	totp, _ := base64.StdEncoding.DecodeString(string(inter))
	totp, _ = api.DecryptData(totp, s.config.GetPassphrase("assemble"))
	user.TotpKey = string(totp)

	var groups []Group = make([]Group, 0)
	var groupsString = s.get(user.ID, USERGROUPS_T)
	g := strings.Split(string(groupsString), ",")

	for _, n := range g {
		log.Infof("Looking for User Group with ID %s", n)
		group, _ := s.FindGroupByID(n)
		groups = append(groups, *group)
	}

	user.Groups = groups

	if user.ID == "" {
		return nil, fmt.Errorf("User %s is not registered", email)
	}
	return &user, nil
}

func (s *Server) DeleteUser(user *User) error {
	var (
		id []byte
		u  User = s.encryptUser(user)
	)

	s.users.Lock()
	defer s.users.Unlock()
	if err := s.users.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(USERS_T))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if 0 == bytes.Compare(v, []byte(u.Email)) {
				id = k
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if id != nil {
		if err := s.users.Db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(USERGROUPS_T))
			b.Delete(id)

			b = tx.Bucket([]byte(TOTP_T))
			b.Delete(id)

			b = tx.Bucket([]byte(PASSW_T))
			b.Delete(id)

			b = tx.Bucket([]byte(USERS_T))
			b.Delete(id)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) AddGroup(group *Group) error {
	if group.ID == "" {
		group.ID = uuid.New().String()
	}

	s.users.Lock()
	defer s.users.Unlock()
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

func (s *Server) FindGroupByID(id string) (*Group, error) {
	var group Group = Group{
		ID: id,
	}

	if err := s.users.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(GROUP_T))
		group.Name = string(b.Get([]byte(id)))

		if group.Name == "" {
			return fmt.Errorf("Unable to find matching group with id %s", id)
		}
		b = tx.Bucket([]byte(GROUP_PERMS_T))
		perms := strings.Split(string(b.Get([]byte(id))), ",")

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

	if group.ID == "" {
		return nil, nil
	}
	return &group, nil
}

func (s *Server) FindGroup(name string) (*Group, error) {
	var group Group = Group{
		Name: name,
	}

	s.users.Lock()
	defer s.users.Unlock()
	if err := s.users.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(GROUP_T))
		c := b.Cursor()
		var id []byte
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if 0 == bytes.Compare(v, []byte(name)) {
				id = k
			}
		}

		if id == nil {
			return fmt.Errorf("Unable to find matching group %s", name)
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

	if group.ID == "" {
		return nil, nil
	}
	return &group, nil
}

func (s *Server) DeleteGroup(name string) error {
	var id []byte
	if err := s.users.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(GROUP_T))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if 0 == bytes.Compare(v, []byte(name)) {
				id = k
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if id != nil {
		s.users.Lock()
		defer s.users.Unlock()
		if err := s.users.Db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(GROUP_PERMS_T))
			b.Delete(id)
			b = tx.Bucket([]byte(GROUP_T))
			b.Delete(id)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) AddPermission(name string) (string, error) {
	var id = uuid.New().String()
	if err := s.users.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PERM_T))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if 0 == bytes.Compare(v, []byte(name)) {
				id = string(k)
				return fmt.Errorf("Permission with name %s already exists", name)
			}
		}
		return nil
	}); err != nil {
		return id, err
	}

	if err := s.users.Db.Update(func(tx *bolt.Tx) error {
		s.users.Lock()
		defer s.users.Unlock()
		b := tx.Bucket([]byte(PERM_T))
		err := b.Put([]byte(id), []byte(name))
		if err != nil {
			return fmt.Errorf("Failed to add or update permission: %s", err)
		}
		return nil
	}); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Server) DeletePermission(name string) error {
	var id []byte
	if err := s.users.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PERM_T))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if 0 == bytes.Compare(v, []byte(name)) {
				id = k
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if id != nil {
		if err := s.users.Db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(PERM_T))
			b.Delete(id)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) FindMachine(address string) string {
	var key string = ""
	s.users.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(MACHINE_TOKENS))
		key = string(b.Get([]byte(address)))
		return nil
	})
	return key
}

func (s *Server) AddMachine(address string) string {
	var (
		key []byte
		err error
	)
	if key, err = s.find(address, MACHINE_TOKENS); err != nil && key == nil {
		var (
			token      string = `{"token": "%s", "validFor": "%s", "expires": "%s"}`
			passphrase string = ""
			expiry     string = fmt.Sprintf("%s", time.Now().AddDate(1, 0, 0))
			pBytes     []byte
			hmac       []byte
		)

		// Flow machines need the passphrase to decrypt data
		// this is about the safest way i can think of to transmit it
		//
		// - First encrypt the passphrase with itself
		// - Calculate the hmac and store it - Flow will retrieve this via IP restricted API call
		// - Repeat the hmac to > encrypted passphrase length (hmachmachmac)
		// - Slice hmac to encrypted passphrase length
		// - XOR encryoted passphrase and slice hmac
		// - wrap into JSON
		// - base64 encode
		pBytes, _ = api.EncryptData([]byte(s.config.Assemble.Passphrase), s.config.Assemble.Passphrase)
		hmac = api.Hmac([]byte(s.config.Assemble.Passphrase), pBytes)
		s.replace(address, MACHINE_HMAC, string(hmac))
		if len(pBytes) > len(hmac) {
			for {
				hmac = bytes.Repeat(hmac, 1)
				if len(hmac) > len(pBytes) {
					break
				}
			}
		}
		hmac = hmac[:len(pBytes)]
		for i := range pBytes {
			pBytes[i] = pBytes[i] ^ hmac[i]
		}
		passphrase = string(pBytes)

		token = fmt.Sprintf(token, passphrase, address, expiry)
		token = base64.StdEncoding.EncodeToString([]byte(token))
		key = []byte(token)
		if err = s.replace(address, MACHINE_TOKENS, token); err != nil {
			log.Error("Failed to add or update machine instance %s, %s", address, err)
			key = []byte("")
		}
	}
	return string(key)
}

func (s *Server) getHmac(ip string) string {
	h, err := s.find(ip, MACHINE_HMAC)
	if h == nil || err != nil {
		return ""
	}
	return string(h)
}

func (s *Server) get(id, where string) string {
	var val string
	if err := s.users.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(where))
		val = string(b.Get([]byte(id)))
		return nil
	}); err != nil {
		return ""
	}
	return val
}

// refactor functions
func (s *Server) find(what, where string) ([]byte, error) {
	var val []byte
	if err := s.users.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(where))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if 0 == bytes.Compare(v, []byte(what)) {
				val = k
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return val, nil
}

func (s *Server) findHashedValue(what, where string) string {
	var id []byte
	if err := s.users.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(where))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if err := bcrypt.CompareHashAndPassword(v, []byte(what)); err == nil {
				id = k
				break
			}
		}
		return nil
	}); err != nil {
		return ""
	}
	return string(id)
}

func (s *Server) replace(what, where, with string) error {
	s.users.Lock()
	defer s.users.Unlock()
	if err := s.users.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(where))
		if err := b.Put([]byte(what), []byte(with)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *Server) delete(what, where string) error {
	s.users.Lock()
	defer s.users.Unlock()
	if err := s.users.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(where))
		if err := b.Delete([]byte(what)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *Server) encryptUser(user *User) User {
	u := User{
		ID:     user.ID,
		Groups: user.Groups,
	}

	bytes, _ := bcrypt.GenerateFromPassword([]byte(user.Email), bcrypt.DefaultCost)
	u.Email = string(bytes)

	bytes, _ = bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	u.Password = string(bytes)

	val, _ := api.EncryptData([]byte(user.TotpKey), s.config.GetPassphrase("assemble"))
	u.TotpKey = base64.StdEncoding.EncodeToString(val)
	return u
}
