package types

import (
	"github.com/TrilliumIT/updog/opentsdb"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type CheckOptions struct {
	Stype      string   `json:"type"`
	HttpMethod string   `json:"http_method"`
	Interval   Interval `json:"interval"`
}

type ServiceStatus struct {
	Instances  map[string]InstanceStatus
	Up         int
	Down       int
	IsDegraded bool
	IsFailed   bool
}

type Service struct {
	Instances       []string      `json:"instances"`
	MaxFailures     int           `json:"max_failures"`
	CheckOptions    *CheckOptions `json:"check_options"`
	instances       map[string]*Instance
	updates         chan *InstanceStatusUpdate
	getInstanceChan chan chan<- map[string]InstanceStatus
}

// this is not exported, and must only be called from within the main loop to avoid concurrent map access
func (s *Service) getStatus() (up, down int, isDegraded, isFailed bool) {
	for _, i := range s.instances {
		if i.status != nil && i.status.Up {
			up++
		}
	}
	down = len(s.instances) - up
	isDegraded = down > 0
	isFailed = down > s.MaxFailures
	return
}

func (s *Service) StartChecks(sTSDBClient *opentsdb.Client) {
	s.updates = make(chan *InstanceStatusUpdate)
	s.getInstanceChan = make(chan chan<- map[string]InstanceStatus)

	if s.CheckOptions == nil {
		s.CheckOptions = &CheckOptions{}
	}

	if s.CheckOptions.Interval == 0 {
		s.CheckOptions.Interval = Interval(10 * time.Second)
	}

	s.instances = make(map[string]*Instance)
	for _, i := range s.Instances {
		s.instances[i] = &Instance{address: i, update: s.updates, status: &InstanceStatus{}}
		if s.CheckOptions.Stype == "" {
			s.CheckOptions.Stype = "tcp_connect"
			if strings.HasPrefix("http", i) {
				s.CheckOptions.Stype = "http_status"
				if s.CheckOptions.HttpMethod == "" {
					s.CheckOptions.HttpMethod = "GET"
				}
			}
		}
	}

	for addr, inst := range s.instances {
		iTSDBClient := sTSDBClient.NewClient(map[string]string{"updog.instance": addr})
		go inst.RunChecks(s.CheckOptions, iTSDBClient)
	}

Status:
	for {
		select {
		case su := <-s.updates:
			l := log.WithField("address", su.address).WithField("status", su.status)
			l.Debug("Recieved status update")
			var i *Instance
			var ok bool
			if i, ok = s.instances[su.address]; !ok {
				l.Warn("Recieved status update for unknown instance")
				continue Status
			}
			i.status = su.status
			instancesUp, instancesDown, isDegraded, isFailed := s.getStatus()
			sTSDBClient.Submit("updog.service.instances_up", instancesUp, su.status.TimeStamp)
			sTSDBClient.Submit("updog.service.instances_down", instancesDown, su.status.TimeStamp)
			sTSDBClient.Submit("updog.service.total_instances", instancesDown+instancesUp, su.status.TimeStamp)
			sTSDBClient.Submit("updog.service.degraded", isDegraded, su.status.TimeStamp)
			sTSDBClient.Submit("updog.service.failed", isFailed, su.status.TimeStamp)
		case gi := <-s.getInstanceChan:
			r := make(map[string]InstanceStatus)
			for k, v := range s.instances {
				r[k] = *v.status
			}
			gi <- r
		}
	}
}

func (s *Service) getInstances() map[string]InstanceStatus {
	rc := make(chan map[string]InstanceStatus)
	s.getInstanceChan <- rc
	return <-rc
}

func (s *Service) GetStatus() ServiceStatus {
	ss := ServiceStatus{}
	ss.Instances = s.getInstances()
	ss.Up = 0
	for _, s := range ss.Instances {
		if s.Up {
			ss.Up++
		}
	}
	ss.Down = len(ss.Instances) - ss.Up
	ss.IsDegraded = ss.Down > 0
	ss.IsFailed = ss.Down > s.MaxFailures

	return ss
}
