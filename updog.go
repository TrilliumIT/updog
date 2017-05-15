package main

import (
	"flag"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"github.com/TrilliumIT/updog/types"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
)

var (
	config *updog.Config
)

func main() {
	var err error
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

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
		app.Name = an
		for sn, service := range app.Services {
			l.WithField("service", sn).Info("Starting checks")
			go service.StartChecks()
		}
	}

	//TODO: start up the http dashboard

	log.Println("Waiting for signal...")
	for s := range sigs {
		log.Debug("Signal Recieved")
		switch s {
		case syscall.SIGHUP:
			for an, app := range config.Applications {
				l := log.WithField("application", an)
				l.Debug("Looping services")
				for sn, s := range app.Services {
					l := l.WithField("service", sn)
					l.Debug("Checking service")
					go func(s *updog.CheckHTTP, l *log.Entry) {
						for in, i := range s.GetInstances() {
							l.WithFields(log.Fields{
								"instance":      in,
								"up":            i.Up,
								"response time": i.ResponseTime,
							}).Info("Instance Status")
						}
					}(s, l)
				}
			}
		default:
			return
		}
	}
}
