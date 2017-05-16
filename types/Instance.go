package types

import (
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

type StatusUpdate struct {
	address string
	status  *Status
}

type Status struct {
	Up           bool
	ResponseTime time.Duration
	TimeStamp    time.Time
}

type Instance struct {
	status  *Status
	address string
	update  chan *StatusUpdate
}

func (i *Instance) CheckTCP(interval time.Duration) {
	log.WithField("address", i.address).Debug("Starting CheckTcp")

	var start time.Time
	t := time.NewTicker(interval)
	for {
		start = time.Now()
		up := tcpConnectCheck(i.address, interval)
		end := time.Now()
		st := &Status{Up: up, ResponseTime: end.Sub(start), TimeStamp: end}
		i.update <- &StatusUpdate{address: i.address, status: st}
		<-t.C
	}
}

func tcpConnectCheck(address string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err == nil {
		defer conn.Close()
	}
	return err == nil
}

func (i *Instance) CheckHTTP(interval time.Duration) {
	log.WithField("address", i.address).Debug("Starting CheckHttp")

	t := time.NewTicker(interval)
	for {
		start := time.Now()
		up := httpStatusCheck(i.address, interval)
		end := time.Now()
		st := &Status{Up: up, ResponseTime: end.Sub(start), TimeStamp: end}
		i.update <- &StatusUpdate{address: i.address, status: st}
		<-t.C
	}
}

func httpStatusCheck(address string, timeout time.Duration) bool {
	client := http.Client{Timeout: timeout}
	resp, err := client.Head(address)
	if err == nil {
		defer resp.Body.Close()
		defer io.Copy(ioutil.Discard, resp.Body)
	}
	return err == nil && resp.StatusCode >= 200 && resp.StatusCode <= 300
}
