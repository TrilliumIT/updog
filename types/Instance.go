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
	TimeStamp    time.Time
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
		end := time.Now()
		i.update <- &Status{address: i.address, Up: err == nil, ResponseTime: end.Sub(start), TimeStamp: end}
		if err == nil {
			conn.Close()
		}
		<-t.C
	}
}

func (i *Instance) CheckHTTP(interval time.Duration) {
	log.WithField("address", i.address).Debug("Starting CheckHttp")

	var up bool
	client := http.Client{Timeout: interval}
	t := time.NewTicker(interval)
	for {
		up = false
		start := time.Now()
		resp, err := client.Head(i.address)
		end := time.Now()
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode <= 399 {
				up = true
			}
			resp.Body.Close()
		}
		i.update <- &Status{address: i.address, Up: up, ResponseTime: end.Sub(start), TimeStamp: end}
		<-t.C
	}
}
