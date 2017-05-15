package updog

import (
	log "github.com/sirupsen/logrus"
	"time"
)

type Service struct {
	Instances    []string `json:"instances"`
	MaxFailures  int      `json:"maxFailures"`
	Stype        string   `json:"type"`
	Interval     Interval `json:"interval"`
	instances    map[string]*Instance
	faults       int
	updates      chan *Status
	getInstances chan chan<- map[string]*Instance
}

func (s *Service) StartChecks() {
	s.updates = make(chan *Status)
	s.getInstances = make(chan chan<- map[string]*Instance)

	s.instances = make(map[string]*Instance)
	for _, i := range s.Instances {
		s.instances[i] = &Instance{Status: Status{address: i}, update: s.updates}
	}

	for addr, inst := range s.instances {
		switch s.Stype {
		case "tcp_connect":
			go inst.CheckTCP(time.Duration(s.Interval))
		case "http_status":
			go inst.CheckHTTP(time.Duration(s.Interval))
			//TODO
			//case "tcp_response":
			//	go s.CheckTcpResponse(addr)
			//case "http_response":
			//	go s.CheckHttpResponse(addr)
		default:
			log.WithField("type", s.Stype).WithField("address", addr).Error("Unknown service type")
		}
	}

Status:
	for {
		select {
		case st := <-s.updates:
			l := log.WithField("status", st)
			l.Debug("Recieved status")
			var i *Instance
			var ok bool
			if i, ok = s.instances[st.address]; !ok {
				l.Warn("Recieved status update for unknown instance")
				break Status
			}
			i.Up = st.Up
			i.ResponseTime = st.ResponseTime
		case gi := <-s.getInstances:
			log.Debug("Instances requested")
			r := make(map[string]*Instance)
			for k, v := range s.instances {
				r[k] = v
			}
			gi <- r
		}
	}
}

func (s *Service) GetInstances() map[string]*Instance {
	rc := make(chan map[string]*Instance)
	log.Debug("Sending update request")
	s.getInstances <- rc
	log.Debug("Waiting on return")
	return <-rc
}

func (s *Service) isUp() bool {
	fails := 0
	for _, i := range s.instances {
		if !i.Up {
			fails++
		}
	}
	return fails <= s.MaxFailures
}

func (s *Service) isDegraded() bool {
	fails := 0
	for _, i := range s.instances {
		if !i.Up {
			fails++
		}
	}
	return fails > 0
}
