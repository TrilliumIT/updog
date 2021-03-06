package types

import (
	"sync"
	"time"
)

const maxApplicationDepth = 2

//Application represents a single application
type Application struct {
	Services   map[string]*Service `json:"services"`
	broker     *applicationBroker
	brokerLock sync.Mutex
}

//ApplicationStatus is the status of an application
type ApplicationStatus struct {
	Services         map[string]ServiceStatus `json:"services"`
	Degraded         bool                     `json:"degraded"`
	Failed           bool                     `json:"failed"`
	ServicesTotal    int                      `json:"services_total"`
	ServicesUp       int                      `json:"services_up"`
	ServicesDegraded int                      `json:"services_degraded"`
	ServicesFailed   int                      `json:"services_failed"`
	InstancesTotal   int                      `json:"instances_total"`
	InstancesUp      int                      `json:"instances_up"`
	InstancesFailed  int                      `json:"instances_failed"`
	TimeStamp        time.Time                `json:"timestamp"`
	LastChange       time.Time                `json:"last_change"`
	idx, cidx        uint64
}

const applicationStatusVariations = 6

func (as *ApplicationStatus) filter(depth uint8) {
	if depth <= 0 {
		as.Services = map[string]ServiceStatus{}
		return
	}

	if depth == 1 {
		for sn, s := range as.Services {
			s.Instances = map[string]InstanceStatus{}
			as.Services[sn] = s
		}
		return
	}
}

func (a *Application) startSubscriptions() { //nolint: dupl
	type serviceStatusUpdate struct {
		name string
		s    ServiceStatus
	}
	updates := make(chan *serviceStatusUpdate)
	for sn, s := range a.Services {
		go func(sn string, s *Service) {
			sub := s.Subscribe(false, 255, 0, false)
			defer sub.Close()
			for ss := range sub.C {
				updates <- &serviceStatusUpdate{name: sn, s: ss}
			}
		}(sn, s)
	}
	go func() {
		lastIdx := make(map[string]uint64)
		var idx, cidx uint64
		for su := range updates {
			idx++
			if lastIdx[su.name] < su.s.cidx {
				lastIdx[su.name] = su.s.cidx
				cidx = idx
			}
			as := ApplicationStatus{
				Services:  map[string]ServiceStatus{su.name: su.s},
				TimeStamp: su.s.TimeStamp,
				idx:       idx,
				cidx:      cidx,
			}
			go func(as ApplicationStatus) { a.broker.notifier <- as }(as)
		}
	}()
}

func (as *ApplicationStatus) recalculate() {
	as.Degraded = false
	as.Failed = false
	as.ServicesTotal = 0
	as.ServicesUp = 0
	as.ServicesDegraded = 0
	as.ServicesFailed = 0
	as.InstancesTotal = 0
	as.InstancesUp = 0
	as.InstancesFailed = 0
	for _, s := range as.Services {
		as.ServicesTotal++
		if !s.Failed && !s.Degraded {
			as.ServicesUp++
		}
		if s.Failed {
			as.Failed = true
			as.ServicesFailed++
		}
		if s.Degraded {
			as.Degraded = true
			as.ServicesDegraded++
		}
		as.InstancesTotal += s.InstancesTotal
		as.InstancesUp += s.InstancesUp
		as.InstancesFailed += s.InstancesFailed
	}
}

func (as *ApplicationStatus) updateFrom(ias *ApplicationStatus) { //nolint: dupl
	if ias.idx > as.idx {
		as.idx = ias.idx
	}
	if ias.cidx > as.cidx {
		as.cidx = ias.cidx
	}
	if as.Services == nil {
		as.Services = make(map[string]ServiceStatus)
	}
	for isn, iss := range ias.Services {
		ass := as.Services[isn]
		ass.updateFrom(&iss)
		ass.recalculate()
		as.Services[isn] = ass
		if as.TimeStamp.Before(ass.TimeStamp) {
			as.TimeStamp = ass.TimeStamp
		}
		if as.LastChange.Before(ass.LastChange) {
			as.LastChange = ass.LastChange
		}
	}
}

func (as *ApplicationStatus) copySummaryFrom(ias *ApplicationStatus) bool {
	if ias.idx > as.idx {
		as.idx = ias.idx
	}
	if ias.cidx > as.cidx {
		as.cidx = ias.cidx
	}
	if as.TimeStamp.Before(ias.TimeStamp) {
		as.TimeStamp = ias.TimeStamp
	}
	c := as.Degraded == ias.Degraded && //nolint: dupl
		as.Failed == ias.Failed &&
		as.ServicesTotal == ias.ServicesTotal &&
		as.ServicesUp == ias.ServicesUp &&
		as.ServicesDegraded == ias.ServicesDegraded &&
		as.ServicesFailed == ias.ServicesFailed &&
		as.InstancesTotal == ias.InstancesTotal &&
		as.InstancesUp == ias.InstancesUp &&
		as.InstancesFailed == ias.InstancesFailed

	if c {
		return false
	}

	as.Degraded = ias.Degraded
	as.Failed = ias.Failed
	as.ServicesTotal = ias.ServicesTotal
	as.ServicesUp = ias.ServicesUp
	as.ServicesDegraded = ias.ServicesDegraded
	as.ServicesFailed = ias.ServicesFailed
	as.InstancesTotal = ias.InstancesTotal
	as.InstancesUp = ias.InstancesUp
	as.InstancesFailed = ias.InstancesFailed
	return true
}

func (as *ApplicationStatus) contains(ias *ApplicationStatus, depth uint8) bool {
	if depth <= 0 {
		return true
	}
	for sn, s := range ias.Services {
		ass, ok := as.Services[sn]
		if !ok {
			return false
		}
		if !s.summaryEquals(&ass) {
			return false
		}
		if !s.contains(&ass, depth-1) {
			return false
		}
	}
	return true
}
