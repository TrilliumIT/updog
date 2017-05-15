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
	var err error
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	var confPath = flag.String("config", "config.yaml", "path to configuration file")
	var debug = flag.Bool("debug", false, "debug logging")
	flag.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	y := []byte(os.Getenv("UPDOG_CONFIG_YAML"))
	if len(y) <= 0 {
		y, err = ioutil.ReadFile(*confPath)
		if err != nil {
			log.WithError(err).WithField("config", confPath).Fatal("Failed to load configuration")
		}
	}

	err = yaml.Unmarshal(y, &config)
	if err != nil {
		log.WithError(err).Fatal("Failed to unmarshal yaml")
	}

	for an, app := range config.Applications {
		l := log.WithField("application", an)
		for sn, service := range app.Services {
			l.WithField("service", sn).Info("Starting checks")
			go func(s updog.Service) {
				s.StartChecks()
			}(service)
		}
	}

	//TODO: start up the http dashboard

	log.Println("Waiting for signal...")
	<-sigs
}
