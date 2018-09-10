package main

import (
	"flag"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	//"time"

	"github.com/TrilliumIT/updog/dashboard"
	"github.com/TrilliumIT/updog/opentsdb"
	updog "github.com/TrilliumIT/updog/types"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
)

func main() {
	var conf *updog.Config = &updog.Config{}
	var err error
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	var confPath = flag.String("config", "config.yaml", "path to configuration file")
	var debug = flag.Bool("debug", false, "debug logging")
	//var subscribe = flag.Bool("subscribe", false, "Subscribe to all events on stdout")
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

	err = yaml.Unmarshal(y, conf)
	if err != nil {
		log.WithError(err).Fatal("Failed to unmarshal yaml")
	}

	if conf.OpenTSDBAddress != "" {
		sourceHost := os.Getenv("OPENTSDB_SOURCE_HOST")
		if sourceHost == "" {
			sourceHost, _ = os.Hostname()
		}
		tsdbClient := opentsdb.NewClient(conf.OpenTSDBAddress, map[string]string{"host": sourceHost})
		err = tsdbClient.Subscribe(conf)
		if err != nil {
			log.WithError(err).Fatal("Failed to subscribe")
		}
	}

	for an, app := range conf.Applications.Applications {
		l := log.WithField("application", an)
		for sn, service := range app.Services {
			l.WithField("service", sn).Info("Starting checks")
			service.StartChecks()
		}
	}

	db := dashboard.NewDashboard(conf)
	go func() {
		err = db.Start()
		if err != nil {
			log.WithError(err).Fatal("Failed to start dashboard")
		}
	}()

	log.Println("Waiting for signal...")
	for range sigs {
		log.Debug("Signal Received")
		return
	}
}
