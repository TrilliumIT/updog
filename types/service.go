package types

import (
	log "github.com/sirupsen/logrus"
	"strings"
	"sync"
	"time"
)

type CheckOptions struct {
	Stype      string   `json:"type"`
	HttpMethod string   `json:"http_method"`
	Interval   Interval `json:"interval"`
}

type Service struct {
	Instances    []*Instance   `json:"instances"`
	MaxFailures  int           `json:"max_failures"`
	CheckOptions *CheckOptions `json:"check_options"`
	broker       *serviceBroker
	brokerLock   sync.Mutex
}

func (s *Service) GetStatus() ServiceStatus {
	sub := s.Subscribe(true)
	defer sub.Close()
	return <-sub.C
}

type ServiceSubscription struct {
	C     chan ServiceStatus
	close chan chan ServiceStatus
}

func (s *Service) Subscribe(full bool) *ServiceSubscription {
	if s.broker == nil {
		s.brokerLock.Lock()
		if s.broker == nil {
			s.broker = newServiceBroker()
		}
		s.brokerLock.Unlock()
	}
	r := &ServiceSubscription{C: make(chan ServiceStatus), close: s.broker.closingClients}
	if full {
		s.broker.newFullClients <- r.C
	} else {
		s.broker.newClients <- r.C
	}
	return r
}

func (s *Service) Sub(full bool) Subscription {
	return s.Subscribe(full)
}

func (s *ServiceSubscription) Close() {
	s.close <- s.C
}

func (s *ServiceSubscription) Next() interface{} {
	return <-s.C
}

type ServiceStatus struct {
	Instances       map[string]InstanceStatus `json:"instances"`
	AvgResponseTime time.Duration             `json:"average_response_time"`
	Degraded        bool                      `json:"degraded"`
	Failed          bool                      `json:"failed"`
	MaxFailures     int                       `json:"max_failures"`
	InstancesTotal  int                       `json:"instances_total"`
	InstancesUp     int                       `json:"instances_up"`
	InstancesFailed int                       `json:"instances_failed"`
}

type serviceBroker struct {
	notifier       chan ServiceStatus
	newClients     chan chan ServiceStatus
	newFullClients chan chan ServiceStatus
	closingClients chan chan ServiceStatus
	clients        map[chan ServiceStatus]struct{}
	fullClients    map[chan ServiceStatus]struct{}
}

func newServiceBroker() *serviceBroker {
	b := &serviceBroker{
		notifier:       make(chan ServiceStatus),
		newClients:     make(chan chan ServiceStatus),
		newFullClients: make(chan chan ServiceStatus),
		closingClients: make(chan chan ServiceStatus),
		clients:        make(map[chan ServiceStatus]struct{}),
		fullClients:    make(map[chan ServiceStatus]struct{}),
	}
	go func() {
		var ss ServiceStatus
		for {
			select {
			case c := <-b.newClients:
				b.clients[c] = struct{}{}
				go func(c chan ServiceStatus, ss ServiceStatus) {
					c <- ss
				}(c, ss)
			case c := <-b.newFullClients:
				b.fullClients[c] = struct{}{}
				go func(c chan ServiceStatus, ss ServiceStatus) {
					c <- ss
				}(c, ss)
			case c := <-b.closingClients:
				delete(b.clients, c)
				delete(b.fullClients, c)
			case iss := <-b.notifier:
				ss = ss.updateInstancesFrom(&iss)
				iss = iss.copySummaryFrom(&ss)
				for c := range b.fullClients {
					go func(c chan ServiceStatus, ss ServiceStatus) {
						c <- ss
					}(c, ss)
				}
				for c := range b.clients {
					go func(c chan ServiceStatus, ss ServiceStatus) {
						c <- iss
					}(c, iss)
				}
			}
		}
	}()
	return b
}

func (s *Service) StartChecks() {
	if s.CheckOptions == nil {
		s.CheckOptions = &CheckOptions{}
	}

	if s.CheckOptions.Interval == 0 {
		s.CheckOptions.Interval = Interval(10 * time.Second)
	}

	if s.broker == nil {
		s.brokerLock.Lock()
		if s.broker == nil {
			s.broker = newServiceBroker()
		}
		s.brokerLock.Unlock()
	}

	type instanceStatusUpdate struct {
		name string
		s    InstanceStatus
	}
	updates := make(chan *instanceStatusUpdate)
	for _, i := range s.Instances {
		if s.CheckOptions.Stype == "" {
			s.CheckOptions.Stype = "tcp_connect"
			if strings.HasPrefix("http", i.address) {
				s.CheckOptions.Stype = "http_status"
				if s.CheckOptions.HttpMethod == "" {
					s.CheckOptions.HttpMethod = "GET"
				}
			}
		}
		i.StartChecks(s.CheckOptions)
		go func(i *Instance) {
			iSub := i.Subscribe()
			defer iSub.Close()
			for is := range iSub.C {
				updates <- &instanceStatusUpdate{s: is, name: i.address}
			}
		}(i)
	}

	go func() {
		for isu := range updates {
			iss := ServiceStatus{Instances: make(map[string]InstanceStatus), MaxFailures: s.MaxFailures}
			l := log.WithField("name", isu.name).WithField("status", isu.s)
			l.Debug("Recieved status update")
			iss.Instances[isu.name] = isu.s
			go func(iss ServiceStatus) { s.broker.notifier <- iss }(iss)
		}
	}()
}

func (ss *ServiceStatus) recalculate() {
	ss.InstancesTotal = 0
	ss.InstancesFailed = 0
	ss.InstancesUp = 0
	ss.AvgResponseTime = time.Duration(0)
	ss.Degraded = false
	ss.Failed = false
	for _, is := range ss.Instances {
		ss.InstancesTotal++
		if is.Up {
			ss.InstancesUp++
		} else {
			ss.InstancesFailed++
			ss.Degraded = true
		}
		ss.AvgResponseTime += is.ResponseTime
	}
	if ss.InstancesTotal > 0 {
		ss.AvgResponseTime = ss.AvgResponseTime / time.Duration(ss.InstancesTotal)
	}
	ss.Failed = ss.InstancesFailed > ss.MaxFailures
	return
}

func (ss ServiceStatus) updateInstancesFrom(iss *ServiceStatus) ServiceStatus {
	ss.MaxFailures = iss.MaxFailures
	if ss.Instances == nil {
		ss.Instances = make(map[string]InstanceStatus)
	}
	for in, i := range iss.Instances {
		ss.Instances[in] = i
	}
	ss.recalculate()
	return ss
}

func (iss ServiceStatus) copySummaryFrom(ss *ServiceStatus) ServiceStatus {
	iss.InstancesTotal = ss.InstancesTotal
	iss.InstancesFailed = ss.InstancesFailed
	iss.InstancesUp = ss.InstancesUp
	iss.AvgResponseTime = ss.AvgResponseTime
	iss.Degraded = ss.Degraded
	iss.Failed = ss.Failed
	return iss
}
