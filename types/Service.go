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
	Instances       map[string]*InstanceStatus `json:"instances"`
	AvgResponseTime time.Duration              `json:"average_response_time"`
	Degraded        bool                       `json:"degraded"`
	Failed          bool                       `json:"failed"`
	MaxFailures     int                        `json:"max_failures"`
	InstancesTotal  int                        `json:"instances_total"`
	InstancesUp     int                        `json:"instances_up"`
	InstancesFailed int                        `json:"instances_failed"`
}

type Service struct {
	Instances        []string      `json:"instances"`
	MaxFailures      int           `json:"max_failures"`
	CheckOptions     *CheckOptions `json:"check_options"`
	instances        map[string]*Instance
	updates          chan *InstanceStatusUpdate
	getServiceStatus chan chan<- *ServiceStatus
}

func (s *Service) StartChecks(sTSDBClient *opentsdb.Client) {
	s.updates = make(chan *InstanceStatusUpdate)
	s.getServiceStatus = make(chan chan<- *ServiceStatus)

	if s.CheckOptions == nil {
		s.CheckOptions = &CheckOptions{}
	}

	if s.CheckOptions.Interval == 0 {
		s.CheckOptions.Interval = Interval(10 * time.Second)
	}

	s.instances = make(map[string]*Instance)
	for _, i := range s.Instances {
		s.instances[i] = &Instance{address: i, update: s.updates}
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
		iTSDBClient := sTSDBClient.NewClient(map[string]string{"instance": addr})
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
			ss := getStatus(s.instances, s.MaxFailures)
			sTSDBClient.Submit("updog.service.degraded", ss.Degraded, su.status.TimeStamp)
			sTSDBClient.Submit("updog.service.failed", ss.Failed, su.status.TimeStamp)
			if ss.InstancesTotal >= len(s.instances) { // Don't send up down and total if we have unknown instances
				sTSDBClient.Submit("updog.service.instances_up", ss.InstancesUp, su.status.TimeStamp)
				sTSDBClient.Submit("updog.service.instances_failed", ss.InstancesFailed, su.status.TimeStamp)
				sTSDBClient.Submit("updog.service.instances_total", ss.InstancesTotal, su.status.TimeStamp)
			}
		case gi := <-s.getServiceStatus:
			gi <- getStatus(s.instances, s.MaxFailures)
		}
	}
}

func (s *Service) GetStatus() *ServiceStatus {
	rc := make(chan *ServiceStatus)
	s.getServiceStatus <- rc
	return <-rc
}

func getStatus(instances map[string]*Instance, maxFailures int) *ServiceStatus {
	ss := &ServiceStatus{MaxFailures: maxFailures, Instances: make(map[string]*InstanceStatus)}
	for in, i := range instances {
		if i.status == nil {
			continue
		}
		ss.InstancesTotal++
		if i.status.Up {
			ss.InstancesUp++
		} else {
			ss.InstancesFailed++
			ss.Degraded = true
		}
		ss.AvgResponseTime += i.status.ResponseTime
		ss.Instances[in] = i.status
	}
	if ss.InstancesTotal > 0 {
		ss.AvgResponseTime = ss.AvgResponseTime / time.Duration(ss.InstancesTotal)
	}
	ss.Failed = ss.InstancesFailed > maxFailures
	return ss
}
