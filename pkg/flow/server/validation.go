// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/crewjam/saml/samlsp"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

/*var (
	validEmail    = regexp.MustCompile(`^[ -~]+@[ -~]+$`)
	validPassword = regexp.MustCompile(`^[ -~]{6,200}$`)
	validString   = regexp.MustCompile(`^[ -~]{1,200}$`)
)*/

func (server *Server) RequireAccount(c *gin.Context) {
	if admin, _ := server.FindGroup("admin"); admin == nil {
		log.Warn("Tiyo flow not configured")
		c.Redirect(http.StatusFound, "/configure")
		return
	}

	if token := c.Request.Header.Get("X-Auth-Token"); token != "" {
		ip := c.ClientIP()
		key := server.FindMachine(ip)
		if token != key {
			c.JSON(http.StatusForbidden, struct {
				Forbidden string
			}{
				Forbidden: "The resource you are trying to access is refused by policy",
			})
		}
		return
	}

	section := strings.Trim(strings.Split(c.Request.RequestURI, "?")[0], "/")
	if section == "login" || section == "logout" {
		return
	}

	current := sessions.Default(c)
	// user has valid session
	if err := server.ValidateSession(current); err == nil {
		return
	}

	var samlSP *samlsp.Middleware = nil
	if server.config.SAML != nil {
		samlSP = server.config.SAML.SamlSP
	}
	if samlSP != nil {
		//active := sessions.Default(c)
		session, err := samlSP.Session.GetSession(c.Request)
		if err != nil {
			log.Debugf("SAML: Unable to get session from requests: %+v", err)
		}

		if session != nil {
			jwtSessionClaims, ok := session.(samlsp.JWTSessionClaims)
			if !ok {
				server.Error(c, http.StatusInternalServerError, fmt.Errorf("unable to decode session into JWTSessionClaims"))
				return
			}

			email := jwtSessionClaims.Subject
			if email == "" {
				server.Error(c, http.StatusInternalServerError, fmt.Errorf("saml: Missing token: email"))
				return
			}

			var jwtGroups []string = jwtSessionClaims.Attributes["Groups"]
			groups := make([]Group, 0)
			for _, g := range jwtGroups {
				group, _ := server.FindGroup(g)
				groups = append(groups, *group)
			}

			user := &User{
				Email:  email,
				Groups: groups,
			}
			server.signinSession(user, c)
			return
		}
	}

	c.Redirect(http.StatusFound, "/login")
}

func (server *Server) ValidateSession(session sessions.Session) error {
	if session.Get("NotBefore") == nil || time.Now().Before(session.Get("NotBefore").(time.Time)) {
		return fmt.Errorf("invalid session (before valid)")
	}

	if session.Get("NotAfter") == nil || time.Now().After(session.Get("NotAfter").(time.Time)) {
		return fmt.Errorf(
			"invalid session (expired session is %s and now is %s)",
			session.Get("NotAfter").(time.Time),
			time.Now())
	}
	return nil
}
