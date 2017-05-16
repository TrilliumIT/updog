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

//go:generate go-bindata -prefix "pub/" -pkg updog -o types/bindata.go pub/...
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

	//bosun := updog.NewBosunClient(conf.BosunAddress)

	for an, app := range conf.Applications {
		l := log.WithField("application", an)
		app.Name = an
		for sn, service := range app.Services {
			l.WithField("service", sn).Info("Starting checks")
			go service.StartChecks()
		}
	}

	//db := &updog.Dashboard{}
	//go db.Start()

	log.Println("Waiting for signal...")
	for s := range sigs {
		log.Debug("Signal Recieved")
		switch s {
		case syscall.SIGHUP:
			appStatus := getApplicationStatus(conf.Applications)
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

func getApplicationStatus(apps map[string]*updog.Application) map[string]*updog.ApplicationStatus {
	type statusUpdate struct {
		an     string
		status *updog.ApplicationStatus
	}
	rc := make(chan statusUpdate)
	defer close(rc)
	for an, app := range apps {
		go func(an string, app *updog.Application) {
			rc <- statusUpdate{an: an, status: app.GetStatus()}
		}(an, app)
	}
	r := make(map[string]*updog.ApplicationStatus)
	for range apps {
		a := <-rc
		r[a.an] = a.status
	}
	return r
}
