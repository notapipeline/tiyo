/**
 * A webserver base GUI and API for managing a server.ApiDB
 *
 * @author Martin Proffitt <mproffitt@choclab.net>
 *
 * Build:
 * go-bindata-assetfs -o web_static.go web/... && go build .
 */
package server

import (
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/gin-contrib/multitemplate"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/mproffitt/tiyo/config"

	log "github.com/sirupsen/logrus"
)

const version = "v0.1.0"

type Server struct {
	Dbname  string
	Port    string
	Address string
	Engine  *gin.Engine
	Api     *Api
	Flags   *flag.FlagSet
	Config  *config.Config
}

func NewServer() *Server {
	server := Server{}
	server.Engine = gin.Default()
	server.Config, _ = config.NewConfig()
	server.Init()
	return &server
}

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

func (server *Server) Run() int {
	log.Info("starting server.Apidb-browser..")

	var err error
	server.Api, err = NewApi(server.Dbname)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	bfs := GetBFS("server/assets/files")
	fmt.Printf("%+v\n", bfs)
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
	server.Engine.GET("/", server.Api.Index)
	server.Engine.GET("/pipeline", server.Api.Index)
	server.Engine.GET("/scan", server.Api.Index)
	server.Engine.GET("/scan/:bucket", server.Api.Index)
	server.Engine.GET("/buckets", server.Api.Index)

	// api methods
	server.Engine.GET("/api/v1/bucket", server.Api.Buckets)
	server.Engine.GET("/api/v1/bucket/:bucket/:child", server.Api.Get)
	server.Engine.GET("/api/v1/bucket/:bucket/:child/*key", server.Api.Get)

	server.Engine.PUT("/api/v1/bucket", server.Api.Put)
	server.Engine.POST("/api/v1/bucket", server.Api.CreateBucket)

	server.Engine.DELETE("/api/v1/bucket/:bucket", server.Api.DeleteBucket)
	server.Engine.DELETE("/api/v1/bucket/:bucket/:child", server.Api.DeleteKey)
	server.Engine.DELETE("/api/v1/bucket/:bucket/:child/*key", server.Api.DeleteKey)

	server.Engine.GET("/api/v1/containers", server.Api.Containers)
	server.Engine.GET("/api/v1/languages", bfs.Languages)

	server.Engine.GET("/api/v1/scan/:bucket", server.Api.PrefixScan)
	server.Engine.GET("/api/v1/scan/:bucket/:child", server.Api.PrefixScan)
	server.Engine.GET("/api/v1/scan/:bucket/:child/*key", server.Api.PrefixScan)
	server.Engine.GET("/api/v1/count/:bucket", server.Api.KeyCount)
	server.Engine.GET("/api/v1/count/:bucket/*child", server.Api.KeyCount)

	if server.Config.Assemble.Cacert != "" && server.Config.Assemble.Cakey != "" {
		err = server.Engine.RunTLS(
			server.Address+":"+server.Port, server.Config.Assemble.Cacert, server.Config.Assemble.Cakey)
	} else {
		err = server.Engine.Run(server.Address + ":" + server.Port)
	}

	if err != nil {
		fmt.Printf("Error: Cannot run server. %+v", err)
		return 1
	}
	return 0
}
