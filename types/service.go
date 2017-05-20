package types

import (
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type CheckOptions struct {
	Stype      string   `json:"type"`
	HttpMethod string   `json:"http_method"`
	Interval   Interval `json:"interval"`
}

type Service struct {
	Instances    []string      `json:"instances"`
	MaxFailures  int           `json:"max_failures"`
	CheckOptions *CheckOptions `json:"check_options"`
	broker       *serviceBroker
}

func (s *Service) GetStatus() ServiceStatus {
	sub := s.Subscribe()
	defer sub.Close()
	return <-sub.C
}

type ServiceSubscription struct {
	C     chan ServiceStatus
	close chan chan ServiceStatus
}

func (s *Service) Subscribe() *ServiceSubscription {
	r := &ServiceSubscription{C: make(chan ServiceStatus), close: s.broker.closingClients}
	s.broker.newClients <- r.C
	return r
}

func (s *ServiceSubscription) Close() {
	s.close <- s.C
	close(s.C)
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
	closingClients chan chan ServiceStatus
	clients        map[chan ServiceStatus]struct{}
}

func newServiceBroker() *serviceBroker {
	b := &serviceBroker{
		notifier:       make(chan ServiceStatus),
		newClients:     make(chan chan ServiceStatus),
		closingClients: make(chan chan ServiceStatus),
		clients:        make(map[chan ServiceStatus]struct{}),
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
			case c := <-b.closingClients:
				delete(b.clients, c)
			case ss = <-b.notifier:
				for c := range b.clients {
					go func(c chan ServiceStatus, ss ServiceStatus) {
						c <- ss
					}(c, ss)
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
		s.broker = newServiceBroker()
	}

	type instanceStatusUpdate struct {
		name string
		s    InstanceStatus
	}
	updates := make(chan *instanceStatusUpdate)
	for _, addr := range s.Instances {
		if s.CheckOptions.Stype == "" {
			s.CheckOptions.Stype = "tcp_connect"
			if strings.HasPrefix("http", addr) {
				s.CheckOptions.Stype = "http_status"
				if s.CheckOptions.HttpMethod == "" {
					s.CheckOptions.HttpMethod = "GET"
				}
			}
		}
		go func(address string, co *CheckOptions) {
			i := NewInstance(address, s.CheckOptions)
			iSub := i.Subscribe()
			defer iSub.Close()
			for is := range iSub.C {
				updates <- &instanceStatusUpdate{s: is, name: address}
			}
		}(addr, s.CheckOptions)
	}

	ss := ServiceStatus{MaxFailures: s.MaxFailures, Instances: make(map[string]InstanceStatus)}
	for isu := range updates {
		l := log.WithField("name", isu.name).WithField("status", isu.s)
		l.Debug("Recieved status update")
		ss.Instances[isu.name] = isu.s
		ss.recalculate()
		go func(ss ServiceStatus) { s.broker.notifier <- ss }(ss)
	}
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
