package updog

import (
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"time"
)

type Status struct {
	address      string
	Up           bool
	ResponseTime time.Duration
}

type Instance struct {
	Status
	update chan *Status
}

func (i *Instance) CheckTCP(interval time.Duration) {
	log.WithField("address", i.address).Debug("Starting CheckTcp")

	var start time.Time
	t := time.NewTicker(interval)
	for {
		start = time.Now()
		conn, err := net.DialTimeout("tcp", i.address, interval)
		i.update <- &Status{address: i.address, Up: err == nil, ResponseTime: time.Now().Sub(start)}
		if err == nil {
			conn.Close()
		}
		<-t.C
	}
}

func (i *Instance) CheckHTTP(interval time.Duration) {
	log.WithField("address", i.address).Debug("Starting CheckHttp")

	var up bool
	var start time.Time
	client := http.Client{Timeout: interval}
	t := time.NewTicker(interval)
	for {
		up = false
		start = time.Now()
		resp, err := client.Head(i.address)
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode <= 399 {
				up = true
			}
			resp.Body.Close()
		}
		i.update <- &Status{address: i.address, Up: up, ResponseTime: time.Now().Sub(start)}
		<-t.C
	}
}
