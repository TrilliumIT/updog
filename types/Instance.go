package types

import (
	"github.com/TrilliumIT/updog/opentsdb"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type InstanceStatusUpdate struct {
	address string
	status  *InstanceStatus
}

type InstanceStatus struct {
	Up           bool
	ResponseTime time.Duration
	TimeStamp    time.Time
}

type Instance struct {
	status  *InstanceStatus
	address string
	update  chan *InstanceStatusUpdate
}

func submitTSDBMetric(tsdbClient *opentsdb.Client, up bool, start, end time.Time) {
	tsdbClient.Submit("updog.instance.up", up, start)
	tsdbClient.Submit("updog.instance.response_time", end.Sub(start), start)
}

func (i *Instance) RunChecks(sType string, interval time.Duration, iTSDBClient *opentsdb.Client) {
	l := log.WithField("address", i.address)
	l.Debug("Starting Checks")
	time.Sleep(time.Duration(rand.Int63n(interval.Nanoseconds())))
	t := time.NewTicker(interval)
	var up bool
	for {
		start := time.Now()
		switch sType {
		case "tcp_connect":
			up = tcpConnectCheck(i.address, interval)
		case "http_status":
			up = httpStatusCheck(i.address, interval)
		default:
			l.WithField("type", sType).Error("Unknown service type")
			return
		}
		end := time.Now()
		submitTSDBMetric(iTSDBClient, up, start, end)
		st := &InstanceStatus{Up: up, ResponseTime: end.Sub(start), TimeStamp: end}
		i.update <- &InstanceStatusUpdate{address: i.address, status: st}
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

func httpStatusCheck(address string, timeout time.Duration) bool {
	client := http.Client{Timeout: timeout}
	resp, err := client.Head(address)
	if err == nil {
		defer resp.Body.Close()
		defer io.Copy(ioutil.Discard, resp.Body)
	}
	return err == nil && resp.StatusCode >= 200 && resp.StatusCode <= 300
}
