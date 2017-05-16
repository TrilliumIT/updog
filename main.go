package main

import (
	"flag"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"github.com/TrilliumIT/updog/dashboard"
	"github.com/TrilliumIT/updog/opentsdb"
	updog "github.com/TrilliumIT/updog/types"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
)

func main() {
	var conf *updog.Config
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

	err = yaml.Unmarshal(y, &conf)
	if err != nil {
		log.WithError(err).Fatal("Failed to unmarshal yaml")
	}

	sourceHost := os.Getenv("OPENTSDB_SOURCE_HOST")
	if sourceHost == "" {
		sourceHost, _ = os.Hostname()
	}
	tsdbClient := opentsdb.NewClient(conf.OpenTSDBAddress, map[string]string{"host": sourceHost})

	for an, app := range conf.Applications {
		l := log.WithField("application", an)
		aTSDBClient := tsdbClient.NewClient(map[string]string{"updog.application": an})
		app.Name = an
		for sn, service := range app.Services {
			l.WithField("service", sn).Info("Starting checks")
			sTSDBClient := aTSDBClient.NewClient(map[string]string{"updog.service": sn})
			go service.StartChecks(sTSDBClient)
		}
	}

	db := dashboard.NewDashboard(&conf.Applications)
	go db.Start()

	log.Println("Waiting for signal...")
	for s := range sigs {
		log.Debug("Signal Recieved")
		switch s {
		case syscall.SIGHUP:
			appStatus := conf.Applications.GetApplicationStatus()
			for an, app := range appStatus {
				l := log.WithField("application", an)
				l.Debug("Looping services")
				for sn, s := range app.Services {
					l := l.WithField("service", sn)
					l.Debug("Checking service")
					for in, i := range s.Instances {
						l := l.WithField("instance", in)
						l.WithFields(log.Fields{
							"up":            i.Up,
							"response time": i.ResponseTime,
							"timestamp":     i.TimeStamp,
						}).Info("Instance Status")
					}
				}
			}
		default:
			return
		}
	}
}
