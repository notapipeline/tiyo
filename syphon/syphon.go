package syphon

import (
	"time"

	"github.com/choclab-net/tiyo/config"
	log "github.com/sirupsen/logrus"
)

type Syphon struct {
	Config *config.Config
}

func NewSyphon() *Syphon {
	syphon := Syphon{}
	var err error
	syphon.Config, err = config.NewConfig()
	if err != nil {
		log.Panic(err)
	}
	return &syphon
}

// Syphon will not have an initialiser. Everything will be through API
func (syphon *Syphon) Init() {}

func (syphon *Syphon) Run() int {
	log.Info("Starting tiyo syphon")
	for {
		log.Info("Polling ", syphon.Config.Assemble.Host, ":", syphon.Config.Assemble.Port)
		time.Sleep(10 * time.Second)
	}
	return 0
}
