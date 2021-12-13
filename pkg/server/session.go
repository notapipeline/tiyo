package server

import (
	b64 "encoding/base64"
	"net/http"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/notapipeline/tiyo/pkg/config"
	"github.com/pquerna/otp/totp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

func (server *Server) Signin(c *gin.Context) {
	web := NewWeb(c, server.config)

	request := make(map[string]string)
	if err := c.ShouldBind(&request); err != nil {
		log.Error("Failed to bind signin session")
		c.Redirect(http.StatusFound, "/signin")
		return
	}
	if c.Request.Method == "POST" && len(request) != 0 {
		log.Debug("Validating signin request")
		if request["email"] != server.config.Admin.Email {
			c.Redirect(http.StatusFound, "/signin?error=invalidemail")
			return
		}

		configPassword, _ := b64.StdEncoding.DecodeString(server.config.Admin.Password)
		if err := bcrypt.CompareHashAndPassword(configPassword, []byte(request["password"])); err != nil {
			c.Redirect(http.StatusFound, "/signin?error=invalidpassword")
			return
		}

		if server.config.Admin.TotpKey != "" && !totp.Validate(request["totp"], server.config.Admin.TotpKey) {
			c.Redirect(http.StatusFound, "/signin?error=invalidpasscode")
			return
		}

		user := config.User{
			Admin: true,
			Email: request["email"],
		}

		if err := server.signinSession(&user, c); err != nil {
			log.Error(err)
		}
		c.Redirect(http.StatusFound, "/")
		return
	}
	if server.shouldRedirect(c) {
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "signin", web)
}

func (server *Server) Signout(c *gin.Context) {
	session := sessions.Default(c)
	session.Set("NotAfter", time.Now())
	session.Set("User", nil)
	session.Clear()
	session.Options(sessions.Options{
		MaxAge: -1,
		Domain: server.config.Assemble.Host,
		Path:   "/",
	})
	session.Save()

	if server.config.SAML.SamlSP != nil {
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     config.SSO_SESSION_NAME,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Domain:   server.config.Assemble.Host,
			Secure:   true,
			MaxAge:   -1,
			Expires:  time.Unix(1, 0),
		})
	}
	c.Redirect(http.StatusFound, "/signin")
}

func (server *Server) signinSession(user *config.User, c *gin.Context) error {
	expires := time.Now().Add(12 * time.Hour)
	session := sessions.Default(c)
	if session != nil {
		session.Set("Admin", user.Admin)
		session.Set("User", *user)
		session.Set("NotBefore", time.Now())
		session.Set("NotAfter", expires)
		session.Options(sessions.Options{
			MaxAge: 60 * 60 * 12,
			Path:   "/",
			Domain: server.config.Assemble.Host,
		})
	}
	err := session.Save()
	return err
}
