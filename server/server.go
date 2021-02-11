// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package server : A webserver base GUI and API for managing a server.APIDB
package server

import (
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/notapipeline/tiyo/config"
	"github.com/gin-contrib/multitemplate"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"

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
	Engine *gin.Engine

	// The API handling requests
	API *API

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
	log.Info("Running in ", mode, " mode")
	if mode != "debug" && mode != "trace" {
		gin.SetMode(gin.ReleaseMode)
	}

	server.Engine = gin.Default()
	server.Config, _ = config.NewConfig()
	server.Init()
	return &server
}

// Init : initialises the server environment
func (server *Server) Init() {
	// Try to load from config file first
	server.Dbname = server.Config.Dbname
	server.Address = server.Config.Assemble.Host
	server.Port = string(server.Config.Assemble.Port)

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

// Run : runs the server component when activated via main
func (server *Server) Run() int {
	log.Info("starting server.APIdb-browser..")

	var (
		err error
		db  string = server.Config.DbDir + "/" + server.Dbname
	)

	server.API, err = NewAPI(db, server.Config)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	bfs := GetBinFileSystem("server/assets/files")
	server.Engine.Use(static.Serve("/static", bfs))

	render := multitemplate.New()
	render.Add("index", LoadTemplates("index.tpl"))
	server.Engine.HTMLRender = render

	server.Engine.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	// page methods
	server.Engine.GET("/", server.API.Index)
	server.Engine.GET("/pipeline", server.API.Index)
	server.Engine.GET("/scan", server.API.Index)
	server.Engine.GET("/scan/:bucket", server.API.Index)
	server.Engine.GET("/buckets", server.API.Index)

	// api methods
	server.Engine.GET("/api/v1/bucket", server.API.Buckets)
	server.Engine.GET("/api/v1/bucket/:bucket/:child", server.API.Get)
	server.Engine.GET("/api/v1/bucket/:bucket/:child/*key", server.API.Get)

	server.Engine.PUT("/api/v1/bucket", server.API.Put)
	server.Engine.POST("/api/v1/bucket", server.API.CreateBucket)

	server.Engine.DELETE("/api/v1/bucket/:bucket", server.API.DeleteBucket)
	server.Engine.DELETE("/api/v1/bucket/:bucket/:child", server.API.DeleteKey)
	server.Engine.DELETE("/api/v1/bucket/:bucket/:child/*key", server.API.DeleteKey)

	server.Engine.GET("/api/v1/containers", server.API.Containers)
	server.Engine.GET("/api/v1/collections/:collection", bfs.Collection)

	server.Engine.GET("/api/v1/scan/:bucket", server.API.PrefixScan)
	server.Engine.GET("/api/v1/scan/:bucket/:child", server.API.PrefixScan)
	server.Engine.GET("/api/v1/scan/:bucket/:child/*key", server.API.PrefixScan)

	server.Engine.GET("/api/v1/count/:bucket", server.API.KeyCount)
	server.Engine.GET("/api/v1/count/:bucket/*child", server.API.KeyCount)

	server.Engine.GET("/api/v1/popqueue/:pipeline/:key", server.API.PopQueue)
	server.Engine.POST("/api/v1/perpetualqueue", server.API.PerpetualQueue)

	server.Engine.GET("/api/v1/status/:pipeline", server.API.FlowStatus)
	server.Engine.POST("/api/v1/execute", server.API.ExecuteFlow)
	server.Engine.POST("/api/v1/startflow", server.API.StartFlow)
	server.Engine.POST("/api/v1/stopflow", server.API.StopFlow)
	server.Engine.POST("/api/v1/destroyflow", server.API.DestroyFlow)
	server.Engine.POST("/api/v1/encrypt", server.API.Encrypt)
	server.Engine.POST("/api/v1/decrypt", server.API.Decrypt)

	host := fmt.Sprintf("%s:%d", server.Config.Assemble.Host, server.Config.Assemble.Port)
	log.Info(host)
	if server.Config.Assemble.Cacert != "" && server.Config.Assemble.Cakey != "" {
		err = server.Engine.RunTLS(host, server.Config.Assemble.Cacert, server.Config.Assemble.Cakey)
	} else {
		err = server.Engine.Run(host)
	}

	if err != nil {
		log.Error("Cannot run server. ", err)
		return 1
	}
	return 0
}
