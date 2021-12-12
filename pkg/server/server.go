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

	"github.com/gin-contrib/multitemplate"
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

	// The API handling requests
	api *api.API

	// Flag set for initialising the assemble server component
	Flags *flag.FlagSet

	// Primary configuration of the assemble server component
	Config *config.Config
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

	return &server
}

// Init : initialises the server environment
func (server *Server) Init() {
	var err error
	if server.Config, err = config.NewConfig(); err != nil {
		log.Error("Failed to load config ", err)
		return
	}

	// Try to load from config file first
	server.Dbname = server.Config.Dbname
	server.Address = server.Config.Assemble.Host
	server.Port = fmt.Sprintf("%d", server.Config.Assemble.Port)

	// Read the static path from the environment if set.
	server.Dbname = os.Getenv("TIYO_DB_NAME")
	server.Port = os.Getenv("TIYO_PORT")
	server.Address = os.Getenv("TIYO_ADDRESS")

	// Use default values if environment not set.
	if server.Port == "" {
		server.Port = "8180"
	}

	server.Flags = flag.NewFlagSet("serve", flag.ExitOnError)
	// Setup for command line processing
	server.Flags.StringVar(&server.Dbname, "d", server.Dbname, "Name of the database")
	server.Flags.StringVar(&server.Port, "p", server.Port, "Port for the web-ui")
	server.Flags.StringVar(&server.Address, "a", server.Address, "Listen address for the web-ui")
	server.Flags.Parse(os.Args[2:])

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
		db  string = server.Config.DbDir + "/" + server.Dbname
	)

	server.api, err = api.NewAPI(db, server.Config)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	bfs := GetBinFileSystem("assets/files")
	server.engine.Use(static.Serve("/static", bfs))

	render := multitemplate.New()
	render.Add("index", LoadTemplates("index.tpl"))
	server.engine.HTMLRender = render

	server.engine.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	// page methods
	server.engine.GET("/", server.Index)
	server.engine.GET("/pipeline", server.Index)
	server.engine.GET("/scan", server.Index)
	server.engine.GET("/scan/:bucket", server.Index)
	server.engine.GET("/buckets", server.Index)

	// api methods
	server.engine.GET("/api/v1/bucket", server.api.Buckets)
	server.engine.GET("/api/v1/bucket/:bucket/:child", server.api.Get)
	server.engine.GET("/api/v1/bucket/:bucket/:child/*key", server.api.Get)

	server.engine.PUT("/api/v1/bucket", server.api.Put)
	server.engine.POST("/api/v1/bucket", server.api.CreateBucket)

	server.engine.DELETE("/api/v1/bucket/:bucket", server.api.DeleteBucket)
	server.engine.DELETE("/api/v1/bucket/:bucket/:child", server.api.DeleteKey)
	server.engine.DELETE("/api/v1/bucket/:bucket/:child/*key", server.api.DeleteKey)

	server.engine.GET("/api/v1/containers", server.api.Containers)
	server.engine.GET("/api/v1/collections/:collection", bfs.Collection)

	server.engine.GET("/api/v1/scan/:bucket", server.api.PrefixScan)
	server.engine.GET("/api/v1/scan/:bucket/:child", server.api.PrefixScan)
	server.engine.GET("/api/v1/scan/:bucket/:child/*key", server.api.PrefixScan)

	server.engine.GET("/api/v1/count/:bucket", server.api.KeyCount)
	server.engine.GET("/api/v1/count/:bucket/*child", server.api.KeyCount)

	server.engine.GET("/api/v1/popqueue/:pipeline/:key", server.api.PopQueue)
	server.engine.POST("/api/v1/perpetualqueue", server.api.PerpetualQueue)

	server.engine.GET("/api/v1/status/:pipeline", server.api.FlowStatus)
	server.engine.POST("/api/v1/execute", server.api.ExecuteFlow)
	server.engine.POST("/api/v1/startflow", server.api.StartFlow)
	server.engine.POST("/api/v1/stopflow", server.api.StopFlow)
	server.engine.POST("/api/v1/destroyflow", server.api.DestroyFlow)
	server.engine.POST("/api/v1/encrypt", server.api.Encrypt)
	server.engine.POST("/api/v1/decrypt", server.api.Decrypt)

	host := fmt.Sprintf("%s:%d", server.Config.Assemble.Host, server.Config.Assemble.Port)
	log.Info(host)
	if server.Config.Assemble.Cacert != "" && server.Config.Assemble.Cakey != "" {
		err = server.engine.RunTLS(host, server.Config.Assemble.Cacert, server.Config.Assemble.Cakey)
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
