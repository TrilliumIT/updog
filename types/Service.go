package types

import (
	"github.com/TrilliumIT/updog/opentsdb"
	log "github.com/sirupsen/logrus"
	"time"
)

type ServiceState uint8

type Service struct {
	Instances    []string `json:"instances"`
	MaxFailures  int      `json:"max_failures"`
	Stype        string   `json:"type"`
	Interval     Interval `json:"interval"`
	instances    map[string]*Instance
	updates      chan *StatusUpdate
	getInstances chan chan<- map[string]Status
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
	s.updates = make(chan *StatusUpdate)
	s.getInstances = make(chan chan<- map[string]Status)

	s.instances = make(map[string]*Instance)
	for _, i := range s.Instances {
		s.instances[i] = &Instance{address: i, update: s.updates}
	}

	for addr, inst := range s.instances {
		iTSDBClient := sTSDBClient.NewClient(map[string]string{"updog.instance": addr})
		go inst.RunChecks(s.Stype, time.Duration(s.Interval), iTSDBClient)
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
		case gi := <-s.getInstances:
			log.Debug("Instances requested")
			r := make(map[string]Status)
			for k, v := range s.instances {
				r[k] = *v.status
			}
			gi <- r
		}
	}
}

func (s *Service) GetInstances() map[string]Status {
	rc := make(chan map[string]Status)
	log.Debug("Sending update request")
	s.getInstances <- rc
	log.Debug("Waiting on return")
	return <-rc
}
