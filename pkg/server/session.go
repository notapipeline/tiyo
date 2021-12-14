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

type signin struct {
	Email    string `form:"email"`
	Password string `form:"password"`
	Otp      string `form:"otpkey,omitempty"`
}

func (server *Server) Signin(c *gin.Context) {
	web := NewWeb(c, server.config)

	request := signin{}
	if err := c.ShouldBind(&request); err != nil {
		log.Error("Failed to bind signin session")
		c.Redirect(http.StatusFound, "/signin")
		return
	}

	if c.Request.Method == "POST" {
		log.Debug("Validating signin request")
		var (
			user *config.User
			err  error
		)

		// blame the user even if the database fails???
		if user, err = server.config.FindUser(request.Email); user == nil || err != nil {
			c.Redirect(http.StatusFound, "/signin?error=invalidemail")
			return
		}

		configPassword, _ := b64.StdEncoding.DecodeString(user.Password)
		if err := bcrypt.CompareHashAndPassword(configPassword, []byte(request.Password)); err != nil {
			c.Redirect(http.StatusFound, "/signin?error=invalidpassword")
			return
		}

		if user.TotpKey != "" && !totp.Validate(request.Otp, user.TotpKey) {
			c.Redirect(http.StatusFound, "/signin?error=invalidpasscode")
			return
		}

		if err := server.signinSession(user, c); err != nil {
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
