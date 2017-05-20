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

type InstanceStatus struct {
	Up           bool          `json:"up"`
	ResponseTime time.Duration `json:"response_time"`
	TimeStamp    time.Time     `json:"timestamp"`
}

type Instance struct {
	broker *instanceBroker
}

type instanceBroker struct {
	notifier       chan *InstanceStatus
	newClients     chan chan *InstanceStatus
	closingClients chan chan *InstanceStatus
	clients        map[chan *InstanceStatus]struct{}
}

type InstanceSubscription struct {
	C     chan InstanceStatus
	close chan chan InstanceStatus
}

func (i *Instance) Subscribe() *InstanceSubscription {
	r := &InstanceSubscription{C: make(chan InstanceStatus), close: i.broker.closingClients}
	i.broker.newClients <- r.C
	return r
}

func (s *InstanceSubscription) Close() {
	s.close <- s.C
	close(s.C)
}

func newInstanceBroker() *instanceBroker {
	b := &instanceBroker{
		notifier:       make(chan InstanceStatus),
		newClients:     make(chan chan InstanceStatus),
		closingClients: make(chan chan InstanceStatus),
		clients:        make(map[chan InstanceStatus]struct{}),
	}
	go func() {
		for {
			select {
			case c := <-b.newClients:
				b.clients[c] = struct{}{}
			case c := <-b.closingClients:
				delete(b.clients, c)
			case is := <-s.notifier:
				for c := range s.clients {
					go func(c chan InstanceStatus, ss InstanceStatus) {
						c <- ss
					}(c, ss)
				}
			}
		}
	}()
}

func NewInstance(address string, co *CheckOptions) *Instance {
	i := &Instance{broker: newInstanceBroker}
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
			up = tcpConnectCheck(address, interval)
		case "http_status":
			up = httpStatusCheck(co.HttpMethod, address, interval)
		default:
			l.WithField("type", co.Stype).Error("Unknown service type")
			return
		}
		end := time.Now()
		st := InstanceStatus{Up: up, ResponseTime: end.Sub(start), TimeStamp: start}
		go func() { i.broker.notifier <- st }()
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
