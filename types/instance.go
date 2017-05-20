package types

import (
	"encoding/json"
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

type Instance struct {
	address string
	broker  *instanceBroker
}

func (i *Instance) UnmarshalJSON(data []byte) (err error) {
	return json.Unmarshal(data, &i.address)
}

func (i Instance) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.address)
}

func (i *Instance) GetStatus() InstanceStatus {
	log.WithField("address", i.address).Debug("Getstatus")
	s := i.Subscribe()
	defer s.Close()
	return <-s.C
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
}

type InstanceStatus struct {
	Up           bool          `json:"up"`
	ResponseTime time.Duration `json:"response_time"`
	TimeStamp    time.Time     `json:"timestamp"`
}

type instanceBroker struct {
	notifier       chan InstanceStatus
	newClients     chan chan InstanceStatus
	closingClients chan chan InstanceStatus
	clients        map[chan InstanceStatus]struct{}
}

func newInstanceBroker() *instanceBroker {
	b := &instanceBroker{
		notifier:       make(chan InstanceStatus),
		newClients:     make(chan chan InstanceStatus),
		closingClients: make(chan chan InstanceStatus),
		clients:        make(map[chan InstanceStatus]struct{}),
	}
	go func() {
		var is InstanceStatus
		for {
			select {
			case c := <-b.newClients:
				log.WithField("b", b).WithField("is", is).Debug("newClient")
				b.clients[c] = struct{}{}
				go func(c chan InstanceStatus, is InstanceStatus) {
					c <- is
				}(c, is)
			case c := <-b.closingClients:
				delete(b.clients, c)
			case is = <-b.notifier:
				log.WithField("b", b).WithField("is", is).Debug("Notified")
				for c := range b.clients {
					go func(c chan InstanceStatus, is InstanceStatus) {
						c <- is
					}(c, is)
				}
			}
		}
	}()
	return b
}

func (i *Instance) StartChecks(co *CheckOptions) {
	log.WithField("Address", i.address).Debug("Starting checks")
	if i.broker == nil {
		i.broker = newInstanceBroker()
	}
	interval := time.Duration(co.Interval)
	go func() {
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
				log.WithField("type", co.Stype).Error("Unknown service type")
				return
			}
			end := time.Now()
			st := InstanceStatus{Up: up, ResponseTime: end.Sub(start), TimeStamp: start}
			go func() { i.broker.notifier <- st }()
			<-t.C
		}
	}()
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
