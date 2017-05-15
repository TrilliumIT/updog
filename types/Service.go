package updog

import (
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"time"
)

type Status Instance

type Service struct {
	Instances   []string `json:"instances"`
	MaxFailures int      `json:"maxFailures"`
	Stype       string   `json:"type"`
	Interval    Interval `json:"interval"`
	instances   map[string]*Instance
	faults      int
	updates     chan *Status
}

func (s *Service) StartChecks() {
	s.updates = make(chan *Status)

	s.instances = make(map[string]*Instance)
	for _, i := range s.Instances {
		s.instances[i] = &Instance{address: i}
	}

	for addr := range s.instances {
		switch s.Stype {
		case "tcp_connect":
			go s.CheckTcp(addr)
		case "http_status":
			go s.CheckHttp(addr)
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
			i.up = st.up
			i.responseTime = st.responseTime
		}
	}
}

func (s *Service) isUp() bool {
	fails := 0
	for _, i := range s.instances {
		if !i.up {
			fails += 1
		}
	}
	return fails <= s.MaxFailures
}

func (s *Service) isDegraded() bool {
	fails := 0
	for _, i := range s.instances {
		if !i.up {
			fails += 1
		}
	}
	return fails > 0
}

func (s *Service) CheckTcp(addr string) {
	log.WithField("address", addr).Debug("Starting CheckTcp")

	var start time.Time
	t := time.NewTicker(time.Duration(s.Interval))
	for {
		start = time.Now()
		conn, err := net.DialTimeout("tcp", addr, time.Duration(s.Interval))
		s.updates <- &Status{address: addr, up: err == nil, responseTime: time.Now().Sub(start)}
		if err == nil {
			conn.Close()
		}
		<-t.C
	}
}

func (s *Service) CheckHttp(addr string) {
	log.WithField("address", addr).Debug("Starting CheckHttp")

	var up bool
	var start time.Time
	client := http.Client{Timeout: time.Duration(s.Interval)}
	t := time.NewTicker(time.Duration(s.Interval))
	for {
		up = false
		start = time.Now()
		resp, err := client.Head(addr)
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode <= 399 {
				up = true
			}
			resp.Body.Close()
		}
		s.updates <- &Status{address: addr, up: up, responseTime: time.Now().Sub(start)}
		<-t.C
	}
}
