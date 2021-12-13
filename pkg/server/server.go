// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package server : A webserver base GUI and API for managing a server.apiDB
package server

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

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
}

// NewServer : Create a new Server instance
func NewServer() *Server {
	server := Server{}
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
		return nil
	}

	log.Info("Running in ", mode, " mode")
	if mode != "debug" && mode != "trace" {
		gin.SetMode(gin.ReleaseMode)
	}

	logfile := filepath.Join(dirname, fmt.Sprintf("%s.log", config.Designate))
	server.engine = gin.New()
	server.engine.Use(Logger(logfile, mode), gin.Recovery())

	server.router = server.engine.Group("/")
	server.router.Use(sessions.Sessions(config.SESSION_COOKIE_NAME, server.securetoken))

	return &server
}

// Init : initialises the server environment
func (server *Server) Init() {
	var err error
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

	server.flags = flag.NewFlagSet("serve", flag.ExitOnError)
	// Setup for command line processing
	server.flags.StringVar(&server.Dbname, "d", server.Dbname, "Name of the database")
	server.flags.StringVar(&server.Port, "p", server.Port, "Port for the web-ui")
	server.flags.StringVar(&server.Address, "a", server.Address, "Listen address for the web-ui")
	server.flags.Parse(os.Args[2:])

	// if dbname is not set by flag or environment, set it as the application basename
	if server.Dbname == "" {
		server.Dbname = fmt.Sprintf("%s.db", path.Base(os.Args[0]))
	}
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
	render.Add("index", LoadTemplates("index.tpl"))
	server.engine.HTMLRender = render

	server.engine.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

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
