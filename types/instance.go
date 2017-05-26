package types

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Instance struct {
	address    string
	broker     *instanceBroker
	brokerLock sync.Mutex
}

func (i *Instance) Address() string {
	return i.address
}

func (i *Instance) UnmarshalJSON(data []byte) (err error) {
	return json.Unmarshal(data, &i.address)
}

func (i Instance) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.address)
}

func (i *Instance) Subscribe(full bool, depth uint8, maxStale time.Duration, onlyChanges bool) *InstanceSubscription {
	if i.broker == nil {
		i.brokerLock.Lock()
		if i.broker == nil {
			i.broker = newInstanceBroker()
		}
		i.brokerLock.Unlock()
	}
	r := &InstanceSubscription{
		C:     make(chan InstanceStatus),
		close: i.broker.closingClients,
		baseSubscription: baseSubscription{
			onlyChanges: onlyChanges,
		},
	}
	i.broker.newClients <- r
	return r
}

func (i *Instance) Sub(full bool, depth uint8, maxStale time.Duration, onlyChanges bool) Subscription {
	return i.Subscribe(full, depth, maxStale, onlyChanges)
}

func (s *InstanceSubscription) Close() {
	s.close <- s.C
}

func (s *InstanceSubscription) Next() interface{} {
	return <-s.C
}

type InstanceStatus struct {
	Up           bool          `json:"up"`
	ResponseTime time.Duration `json:"response_time"`
	TimeStamp    time.Time     `json:"timestamp"`
	LastChange   time.Time     `json:"last_change"`
	idx, cidx    uint64
}

type instanceBroker struct {
	notifier       chan InstanceStatus
	newClients     chan *InstanceSubscription
	closingClients chan chan InstanceStatus
	clients        map[chan InstanceStatus]*InstanceSubscription
}

func newInstanceBroker() *instanceBroker {
	b := &instanceBroker{
		notifier:       make(chan InstanceStatus),
		newClients:     make(chan *InstanceSubscription),
		closingClients: make(chan chan InstanceStatus),
		clients:        make(map[chan InstanceStatus]*InstanceSubscription),
	}
	go func() {
		var is InstanceStatus
		for {
			select {
			case c := <-b.newClients:
				log.WithField("b", b).WithField("is", is).Debug("newClient")
				b.clients[c.C] = c
				if is.idx > 0 {
					go func(c chan InstanceStatus, is InstanceStatus) {
						c <- is
					}(c.C, is)
				}
			case c := <-b.closingClients:
				delete(b.clients, c)
			case is = <-b.notifier:
				log.WithField("b", b).WithField("is", is).Debug("Notified")
				for c, o := range b.clients {
					if !o.onlyChanges || o.lastIdx < is.cidx {
						go func(c chan InstanceStatus, is InstanceStatus) {
							c <- is
						}(c, is)
					}
				}
			}
		}
	}()
	return b
}

func (i *Instance) startSubscriptions() {
}

func (i *Instance) StartChecks(co *CheckOptions) {
	log.WithField("Address", i.address).Debug("Starting checks")

	if i.broker == nil {
		i.brokerLock.Lock()
		if i.broker == nil {
			i.broker = newInstanceBroker()
		}
		i.brokerLock.Unlock()
	}

	interval := time.Duration(co.Interval)
	go func() {
		time.Sleep(time.Duration(rand.Int63n(interval.Nanoseconds())))
		t := time.NewTicker(interval)
		var up, lastUp bool
		var idx, cidx uint64
		var start, end time.Time
		for {
			idx++
			start = time.Now()
			switch co.Stype {
			case "tcp_connect":
				up = tcpConnectCheck(i.address, interval)
			case "http_status":
				up = httpStatusCheck(co.HttpMethod, i.address, interval)
			default:
				log.WithField("type", co.Stype).Error("Unknown service type")
				return
			}
			end = time.Now()
			if up != lastUp {
				lastUp = up
				cidx = idx
			}
			go func(st InstanceStatus) { i.broker.notifier <- st }(InstanceStatus{
				Up:           up,
				ResponseTime: end.Sub(start),
				TimeStamp:    start,
				idx:          idx,
				cidx:         cidx,
			})
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
