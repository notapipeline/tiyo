// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
package server

import (
	"net/http"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/notapipeline/tiyo/pkg/config"
	"golang.org/x/crypto/bcrypt"
)

func (server *Server) Signin(c *gin.Context) {
	if c.Request.Method == http.MethodPost {
		request := login{}
		if err := c.ShouldBind(&request); err != nil {
			server.Error(c, 500, err)
			return
		}

		user, err := server.FindUser(request.Email)
		if err != nil {
			server.Error(c, 500, err)
			return
		}
		if user == nil {
			c.Redirect(http.StatusFound, "/login?error=notfound")
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.Password)); err != nil {
			c.Redirect(http.StatusFound, "/login?error=invalidpassword")
			return
		}

		expires := time.Now().Add(12 * time.Hour)
		session := sessions.Default(c)
		session.Set("User", *user)
		session.Set("NotBefore", time.Now())
		session.Set("NotAfter", expires)
		session.Options(sessions.Options{
			MaxAge: 60 * 60 * 12,
			Path:   "/",
			Domain: server.config.Flow.Hostname,
		})
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}
	c.HTML(200, "login", gin.H{
		"Title": "TIYO - Log in to the cluster designer",
	})
}

func (server *Server) Signout(c *gin.Context) {
	session := sessions.Default(c)
	session.Set("NotAfter", time.Now())
	session.Set("User", nil)
	session.Clear()
	session.Options(sessions.Options{
		MaxAge: -1,
		Domain: server.config.Flow.Hostname,
		Path:   "/",
	})
	session.Save()

	if server.config.SAML != nil && server.config.SAML.SamlSP != nil {
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     config.SSO_SESSION_NAME,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Domain:   server.config.Flow.Hostname,
			Secure:   true,
			MaxAge:   -1,
			Expires:  time.Unix(1, 0),
		})
	}
	c.Redirect(http.StatusFound, "/login")
}

func (server *Server) signinSession(user *User, c *gin.Context) error {
	expires := time.Now().Add(12 * time.Hour)
	session := sessions.Default(c)
	session.Set("User", *user)
	session.Set("NotBefore", time.Now())
	session.Set("NotAfter", expires)
	session.Options(sessions.Options{
		MaxAge: 60 * 60 * 12,
		Path:   "/",
		Domain: server.config.Flow.Hostname,
	})
	err := session.Save()
	return err
}
