package main

import (
	"flag"
	"github.com/TrilliumIT/updog/types"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
)

var (
	config *updog.Config
)

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	var confPath = flag.String("config", "config.yaml", "path to configuration file")
	flag.Parse()

	y, err := ioutil.ReadFile(*confPath)
	if err != nil {
		log.Fatalf("failed to load configuration from: %v", confPath)
	}

	err = yaml.Unmarshal(y, &config)
	if err != nil {
		log.Fatalf("failed to unmarshal yaml: %v", err.Error())
	}

	for an, app := range config.Applications {
		for sn, service := range app.Services {
			log.Infof("Starting checks for service %v in app %v.", sn, an)
			go func(s updog.Service) {
				s.StartChecks()
			}(service)
		}
	}

	//TODO: start up the http dashboard

	log.Println("Waiting for signal...")
	<-sigs
}
