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

type ServiceStatus struct {
	Instances       map[string]InstanceStatus `json:"instances"`
	AvgResponseTime time.Duration             `json:"average_response_time"`
	Degraded        bool                      `json:"degraded"`
	Failed          bool                      `json:"failed"`
	MaxFailures     int                       `json:"max_failures"`
	InstancesTotal  int                       `json:"instances_total"`
	InstancesUp     int                       `json:"instances_up"`
	InstancesFailed int                       `json:"instances_failed"`
	TimeStamp       time.Time                 `json:"timestamp"`
	LastChange      time.Time                 `json:"last_change"`
	idx, cidx       uint64
}

func (ss *ServiceStatus) filter(depth uint8) {
	if depth <= 0 {
		ss.Instances = map[string]InstanceStatus{}
	}
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
			iSub := i.Subscribe(true, 255, 0, false)
			defer iSub.Close()
			var lc time.Time
			var ls bool
			for is := range iSub.C {
				if is.Up != ls {
					ls = is.Up
					lc = is.TimeStamp
				}
				is.LastChange = lc
				updates <- &instanceStatusUpdate{s: is, name: i.address}
			}
		}(i)
	}

	go func() {
		lastIdx := make(map[string]uint64)
		var idx, cidx uint64
		for isu := range updates {
			idx++
			if lastIdx[isu.name] < isu.s.cidx {
				lastIdx[isu.name] = isu.s.cidx
				cidx = idx
			}
			l := log.WithField("name", isu.name).WithField("status", isu.s)
			l.Debug("Recieved status update")
			iss := ServiceStatus{
				Instances:   map[string]InstanceStatus{isu.name: isu.s},
				MaxFailures: s.MaxFailures,
				idx:         idx,
				cidx:        cidx,
			}
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

func (ss *ServiceStatus) updateFrom(iss *ServiceStatus) {
	if iss.idx > ss.idx {
		ss.idx = iss.idx
	}
	if iss.cidx > ss.cidx {
		ss.cidx = iss.cidx
	}
	ss.MaxFailures = iss.MaxFailures
	if ss.Instances == nil {
		ss.Instances = make(map[string]InstanceStatus)
	}
	for in, i := range iss.Instances {
		ss.Instances[in] = i
		if ss.TimeStamp.Before(i.TimeStamp) {
			ss.TimeStamp = i.TimeStamp
		}
		if ss.LastChange.Before(i.LastChange) {
			ss.LastChange = i.LastChange
		}
	}
}

func (iss *ServiceStatus) summaryEquals(ss *ServiceStatus) bool {
	return iss.Degraded == ss.Degraded &&
		iss.Failed == ss.Failed &&
		iss.InstancesTotal == ss.InstancesTotal &&
		iss.InstancesFailed == ss.InstancesFailed &&
		iss.InstancesUp == ss.InstancesUp &&
		iss.AvgResponseTime == ss.AvgResponseTime
}

func (iss *ServiceStatus) copySummaryFrom(ss *ServiceStatus) bool {
	if ss.idx > iss.idx {
		iss.idx = ss.idx
	}
	if ss.cidx > iss.cidx {
		iss.cidx = ss.cidx
	}
	if iss.TimeStamp.Before(ss.TimeStamp) {
		iss.TimeStamp = ss.TimeStamp
	}
	if iss.summaryEquals(ss) {
		return false
	}

	iss.Degraded = ss.Degraded
	iss.Failed = ss.Failed
	iss.InstancesTotal = ss.InstancesTotal
	iss.InstancesFailed = ss.InstancesFailed
	iss.InstancesUp = ss.InstancesUp
	iss.AvgResponseTime = ss.AvgResponseTime
	iss.Degraded = ss.Degraded
	iss.Failed = ss.Failed
	return true
}

func (ss *ServiceStatus) contains(iss *ServiceStatus, depth uint8) bool {
	if depth <= 0 {
		return true
	}
	for in, i := range iss.Instances {
		ssi, ok := ss.Instances[in]
		if !ok {
			return false
		}
		if i.TimeStamp != ssi.TimeStamp {
			return false
		}
		if i.ResponseTime != ssi.ResponseTime {
			return false
		}
		if i.Up != ssi.Up {
			return false
		}
	}
	return true
}
