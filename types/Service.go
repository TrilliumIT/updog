package updog

import (
	"log"
	"net"
	"net/http"
	"time"
)

type Result struct {
	address string
	up      bool
}

type Service struct {
	Instances   []string `json:"instances"`
	MaxFailures int      `json:"maxFailures"`
	Stype       string   `json:"type"`
	Interval    Interval `json:"interval"`
	instances   map[string]bool
	faults      int
	updates     chan *Result
}

func (s *Service) StartChecks() {
	s.updates = make(chan *Result)

	s.instances = make(map[string]bool)
	for _, i := range s.Instances {
		s.instances[i] = false
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
			log.Println("Encountered unknown service type: %v for address: %v.\n", s.Stype, addr)
		}
	}

	for {
		select {
		case r := <-s.updates:
			log.Printf("Recieved update: %v.\n", r)
			s.instances[r.address] = r.up
		}
	}
}

func (s *Service) isUp() bool {
	fails := 0
	for _, i := range s.instances {
		if !i {
			fails += 1
		}
	}
	return fails <= s.MaxFailures
}

func (s *Service) isDegraded() bool {
	fails := 0
	for _, i := range s.instances {
		if !i {
			fails += 1
		}
	}
	return fails > 0
}

func (s *Service) CheckTcp(addr string) {
	log.Printf("Starting CheckTcp() for %v.\n", addr)
	t := time.NewTicker(time.Duration(s.Interval))
	for {
		conn, err := net.DialTimeout("tcp", addr, time.Duration(s.Interval))
		s.updates <- &Result{address: addr, up: err == nil}
		if err == nil {
			conn.Close()
		}
		<-t.C
	}
}

func (s *Service) CheckHttp(addr string) {
	log.Printf("Starting CheckHttp() for %v.\n", addr)
	t := time.NewTicker(time.Duration(s.Interval))
	for {
		up := false
		client := http.Client{Timeout: time.Duration(s.Interval)}
		resp, err := client.Head(addr)
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode <= 399 {
				up = true
			}
			resp.Body.Close()
		}
		s.updates <- &Result{address: addr, up: up}
		<-t.C
	}
}
