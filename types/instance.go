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
	Up           bool          `json:"up"`
	ResponseTime time.Duration `json:"response_time"`
	TimeStamp    time.Time     `json:"timestamp"`
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

func (i *Instance) RunChecks(co *CheckOptions, iTSDBClient *opentsdb.Client) {
	interval := time.Duration(co.Interval)
	l := log.WithField("address", i.address)
	l.Debug("Starting Checks")
	time.Sleep(time.Duration(rand.Int63n(interval.Nanoseconds())))
	t := time.NewTicker(interval)
	var up bool
	for {
		start := time.Now()
		switch co.Stype {
		case "tcp_connect":
			up = tcpConnectCheck(i.address, interval)
		case "http_status":
			up = httpStatusCheck(co.HttpMethod, i.address, interval)
		default:
			l.WithField("type", co.Stype).Error("Unknown service type")
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

func httpStatusCheck(method, address string, timeout time.Duration) bool {
	l := log.WithFields(log.Fields{"method": method, "address": address, "timeout": timeout})
	client := http.Client{Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	req, err := http.NewRequest(method, address, nil)
	if err != nil {
		l.WithError(err).Error("Failed to create http request.")
	}
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		defer io.Copy(ioutil.Discard, resp.Body)
	}
	return err == nil && resp.StatusCode >= 200 && resp.StatusCode <= 399
}
