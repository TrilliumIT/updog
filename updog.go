package main

import (
	"github.com/TrilliumIT/updog/types"
	"github.com/ghodss/yaml"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	y, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Fatal("failed to load config.yaml")
	}

	var config updog.Config

	err = yaml.Unmarshal(y, &config)
	if err != nil {
		log.Fatalf("failed to unmarshal yaml: %v", err.Error())
	}

	for an, app := range config.Applications {
		for sn, service := range app.Services {
			log.Printf("Starting checks for service %v in app %v.\n", sn, an)
			go func(s updog.Service) {
				s.StartChecks()
			}(service)
		}
	}

	log.Println("Waiting for signal...")
	<-sigs
}
