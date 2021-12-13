package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/crewjam/saml/samlsp"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/notapipeline/tiyo/pkg/config"
	"github.com/pquerna/otp"
	log "github.com/sirupsen/logrus"
)

type Web struct {
	// Internal
	w        http.ResponseWriter
	r        *http.Request
	ps       gin.Params
	template string

	// Default
	Backlink string
	Version  string
	Request  *http.Request
	Section  string
	Time     time.Time
	Admin    bool
	SamlM    *samlsp.Middleware
	Saml     config.SAML
	Info     config.Admin
	User     config.User
	Errors   []string

	SemanticTheme string
	TempTotpKey   *otp.Key
}

func NewWeb(c *gin.Context, conf *config.Config) *Web {
	section := strings.Trim(strings.Split(c.Request.RequestURI, "?")[0], "/")
	web := Web{
		w:  c.Writer,
		r:  c.Request,
		ps: c.Params,

		Backlink: "/",
		Version:  "",
		Request:  c.Request,
		Section:  section,
		Time:     time.Now(),
		SamlM:    conf.SAML.SamlSP,
		Saml:     *conf.SAML,
		Info:     *conf.Admin,
		Errors:   make([]string, 0),
	}

	if _, ok := c.Get(sessions.DefaultKey); ok {
		session := sessions.Default(c)
		if session.Get("Admin") != nil {
			web.Admin = session.Get("Admin").(bool)
		}
	}
	return &web
}

func (w *Web) Error(err error) {
	log.Error(err)
	w.Errors = append(w.Errors, err.Error())
}
