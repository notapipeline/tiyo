// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package server : A webserver base GUI and API for managing a server.apiDB
package server

import (
	"encoding/gob"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-contrib/multitemplate"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/notapipeline/tiyo/pkg/config"
	"github.com/notapipeline/tiyo/pkg/server/api"

	log "github.com/sirupsen/logrus"
)

// Server : The principle server component
type Server struct {

	// The name of the database file
	Dbname string

	// The port to listen on
	Port string

	// The address of the server component
	Address string

	// The GIN HTTP Engine
	engine *gin.Engine

	// Router for authenticated endpoints
	router *gin.RouterGroup

	// Cookie store for session data
	securetoken cookie.Store

	// The API handling requests
	api *api.API

	// Flag set for initialising the assemble server component
	flags *flag.FlagSet

	// Primary configuration of the assemble server component
	config *config.Config

	// Users database
	users *Lockable
}

// NewServer : Create a new Server instance
func NewServer() *Server {
	s := Server{}
	mode := os.Getenv("TIYO_LOG")
	if mode == "" {
		mode = "production"
	}
	gin.DisableConsoleColor()
	// create log directory if it does not exist
	var dirname string = "/var/log/tiyo"
	if fi, err := os.Stat(dirname); err != nil || !fi.IsDir() {
		if !os.IsNotExist(err) && !fi.IsDir() {
			return nil
		}
	}

	if err := os.Mkdir(dirname, os.ModePerm); err != nil && !os.IsExist(err) {
		log.Fatal(err)
		return nil
	}

	log.Info("Running in ", mode, " mode")
	if mode != "debug" && mode != "trace" {
		gin.SetMode(gin.ReleaseMode)
	}

	logfile := filepath.Join(dirname, fmt.Sprintf("%s.log", config.Designate))
	s.engine = gin.New()
	s.engine.Use(Logger(logfile, mode), gin.Recovery())

	gob.Register(time.Time{})
	gob.Register(User{})

	return &s
}

// Init : initialises the server environment
func (server *Server) Init() {
	var (
		err error
		db  *bolt.DB
	)

	if server.config, err = config.NewConfig(); err != nil {
		log.Error("Failed to load config ", err)
		return
	}

	// Try to load from config file first
	server.Dbname = server.config.Dbname
	server.Address = server.config.Assemble.Host
	server.Port = fmt.Sprintf("%d", server.config.Assemble.Port)

	// Read the static path from the environment if set.
	server.Dbname = os.Getenv("TIYO_DB_NAME")
	server.Port = os.Getenv("TIYO_PORT")
	server.Address = os.Getenv("TIYO_ADDRESS")

	// Use default values if environment not set.
	if server.Port == "" {
		server.Port = "8180"
	}

	server.flags = flag.NewFlagSet("assemble", flag.ExitOnError)
	// Setup for command line processing
	server.flags.StringVar(&server.Dbname, "d", server.Dbname, "Name of the database")
	server.flags.StringVar(&server.Port, "p", server.Port, "Port for the web-ui")
	server.flags.StringVar(&server.Address, "a", server.Address, "Listen address for the web-ui")
	server.flags.Parse(os.Args[2:])

	// if dbname is not set by flag or environment, set it as the application basename
	if server.Dbname == "" {
		server.Dbname = fmt.Sprintf("%s.db", path.Base(os.Args[0]))
	}

	var usersDbName string = filepath.Join(server.config.DbDir, "users.db")
	db, err = bolt.Open(usersDbName, 0600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		log.Error("Failed to open users database", err)
		return
	}

	server.users = &Lockable{
		Db: db,
	}

	if err := server.CreateTables(); err != nil {
		log.Error("Failed to setup database")
		return
	}

	server.securetoken = cookie.NewStore([]byte(config.SESSION_COOKIE_NAME))
	server.securetoken.Options(sessions.Options{
		MaxAge:   60 * 60 * 12,
		Secure:   true,
		HttpOnly: true,
		Domain:   server.config.Assemble.Host,
		Path:     "/",
	})

	server.router = server.engine.Group("/")
	server.router.Use(sessions.Sessions(config.SESSION_COOKIE_NAME, server.securetoken))

}

func (server *Server) Engine() *gin.Engine {
	return server.engine
}

// Run : runs the server component when activated via main
func (server *Server) Run() int {
	log.Info("starting server.apidb-browser..")

	var (
		err error
		db  string = server.config.DbDir + "/" + server.Dbname
	)

	server.api, err = api.NewAPI(db, server.config)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	bfs := GetBinFileSystem("assets/files")
	server.engine.Use(static.Serve("/static", bfs))

	server.setupRoutes(bfs)

	render := multitemplate.New()
	render.Add("index", LoadTemplates("index.tpl", "header.html", "footer.html"))
	render.Add("configure", LoadTemplates("configure.tpl", "header.html", "footer.html"))
	render.Add("login", LoadTemplates("login.tpl", "header.html", "footer.html"))
	render.Add("error", LoadTemplates("error.tpl", "header.html", "footer.html"))
	server.engine.HTMLRender = render

	host := fmt.Sprintf("%s:%d", server.config.Assemble.Host, server.config.Assemble.Port)
	log.Info(host)
	if server.config.Assemble.Cacert != "" && server.config.Assemble.Cakey != "" {
		err = server.engine.RunTLS(host, server.config.Assemble.Cacert, server.config.Assemble.Cakey)
	} else {
		err = server.engine.Run(host)
	}

	if err != nil {
		log.Error("Cannot run server. ", err)
		return 1
	}
	return 0
}

// Index : Render the index page back on the GIN context
//
// TODO : Make this a little more versatile and use SSR to render
//        more of the page than relying on JS and a one page website
func (server *Server) Index(c *gin.Context) {
	c.HTML(200, "index", gin.H{
		"Title": "TIYO ASSEMBLE - Kubernetes cluster designer",
	})
}

// Display any errors back to the user
func (server *Server) Error(c *gin.Context, code int, err error) {
	log.Error(err)
	c.HTML(code, "error", err.Error())
}

// If we hit a non-session page when we should be in session (e.g. signin)
func (server *Server) shouldRedirect(c *gin.Context) bool {
	if _, ok := c.Get(sessions.DefaultKey); ok {
		session := sessions.Default(c)
		return session.Get("NotAfter") != nil && time.Now().Before(session.Get("NotAfter").(time.Time))
	}
	return false
}

type login struct {
	Email    string `form:"email"`
	Password string `form:"password"`
	Confirm  string `form:"confirmPassword,omitempty"`
	Otp      string `form:"otp,omitempty"`
}

func (server *Server) Configure(c *gin.Context) {
	if c.Request.Method == http.MethodPost {
		request := login{}
		if err := c.ShouldBind(&request); err != nil {
			server.Error(c, 500, err)
		}

		if request.Password != request.Confirm {
			c.Redirect(http.StatusFound, "/configure?error=invalidpassword")
			return
		}

		id, _ := server.AddPermission("admin")
		perms := make([]Permission, 0)
		perms = append(perms, Permission{
			ID: id,
		})

		server.AddGroup(&Group{
			Name:        "admin",
			Permissions: perms,
		})

		adminGroups := make([]Group, 0)
		g, err := server.FindGroup("admin")
		if err != nil {
			log.Errorf("Unable to find admin group : %+v", err)
		}

		adminGroups = append(adminGroups, *g)

		var user User = User{
			Email:    request.Email,
			Password: request.Password,
			TotpKey:  request.Otp,
			Groups:   adminGroups,
		}
		if err := server.AddUser(&user); err != nil {
			server.Error(c, http.StatusInternalServerError, err)
		}

		c.Redirect(http.StatusFound, "/")
		return
	}
	c.HTML(200, "configure", gin.H{
		"Title": "TIYO - Kubernetes made easy",
	})
}

var forbidden struct{ Forbidden string } = struct{ Forbidden string }{
	Forbidden: "You have attempted to access a restricted endpoint, this interaction has been recorded",
}

func (s *Server) addmachine(c *gin.Context) {
	var client string = c.ClientIP()
	external, _ := externalIP()
	if client != "127.0.0.1" && (external != "" && client != external) {
		log.Warnf("Attempt to access addmachine endpoint by unknown client IP %s", client)
		c.JSON(http.StatusForbidden, forbidden)
		return
	}

	var address struct {
		Address string `json:"address"`
	}
	if err := c.ShouldBind(&address); err != nil || address.Address == "" {
		log.Errorf("Invalid JSON object - address not provided : %s : %+v", err, address)
		c.JSON(http.StatusBadRequest, struct{ BadRequest string }{BadRequest: "Address is required"})
		return
	}

	var key string = s.AddMachine(address.Address)
	c.JSON(http.StatusOK, struct{ Key string }{Key: key})
}

func (s *Server) hmac(c *gin.Context) {
	client := c.ClientIP()
	hmac := s.getHmac(client)
	if hmac == "" {
		log.Warnf("Attempt to access HMAC endpoint by unknown client IP %s", client)
		c.JSON(http.StatusForbidden, forbidden)
		return
	}
	c.JSON(http.StatusOK, struct {
		Hmac string `json:"hmac"`
	}{Hmac: hmac})
}
