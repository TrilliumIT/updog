package types

import (
	"encoding/json"
	"sync"
	"time"
)

const maxApplicationsDepth = 3

//Applications represents multiple Application objects
type Applications struct {
	Applications map[string]*Application
	broker       *applicationsBroker
	brokerLock   sync.Mutex
}

//UnmarshalJSON unmarshals the JSON bytes
func (a *Applications) UnmarshalJSON(data []byte) (err error) {
	return json.Unmarshal(data, &a.Applications)
}

//MarshalJSON marshals the data structure to a byte array
func (a *Applications) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.Applications)
}

//ApplicationsStatus is the overall status of the Application objects
type ApplicationsStatus struct {
	Applications         map[string]ApplicationStatus `json:"applications"`
	Degraded             bool                         `json:"degraded"`
	Failed               bool                         `json:"failed"`
	ApplicationsTotal    int                          `json:"applications_total"`
	ApplicationsUp       int                          `json:"applications_up"`
	ApplicationsDegraded int                          `json:"applications_degraded"`
	ApplicationsFailed   int                          `json:"applications_failed"`
	ServicesTotal        int                          `json:"services_total"`
	ServicesUp           int                          `json:"services_up"`
	ServicesDegraded     int                          `json:"services_degraded"`
	ServicesFailed       int                          `json:"services_failed"`
	InstancesTotal       int                          `json:"instances_total"`
	InstancesUp          int                          `json:"instances_up"`
	InstancesFailed      int                          `json:"instances_failed"`
	TimeStamp            time.Time                    `json:"timestamp"`
	LastChange           time.Time                    `json:"last_change"`
	idx, cidx            uint64
}

const applicationsStatusVariations = 8

func (as *ApplicationsStatus) filter(depth uint8) {
	if depth <= 0 {
		as.Applications = map[string]ApplicationStatus{}
		return
	}

	if depth == 1 {
		for an, a := range as.Applications {
			a.Services = map[string]ServiceStatus{}
			as.Applications[an] = a
		}
		return
	}

	if depth == 2 {
		for an, a := range as.Applications {
			for sn, s := range a.Services {
				s.Instances = map[string]InstanceStatus{}
				as.Applications[an].Services[sn] = s
			}
		}
	}
}

func (a *Applications) startSubscriptions() { //nolint: dupl
	type applicationStatusUpdate struct {
		name string
		s    ApplicationStatus
	}
	updates := make(chan *applicationStatusUpdate)
	for an, a := range a.Applications {
		go func(an string, a *Application) {
			sub := a.Subscribe(false, 255, 0, false)
			defer sub.Close()
			for as := range sub.C {
				updates <- &applicationStatusUpdate{name: an, s: as}
			}
		}(an, a)
	}
	go func() {
		lastIdx := make(map[string]uint64)
		var idx, cidx uint64
		for au := range updates {
			idx++
			if lastIdx[au.name] < au.s.cidx {
				lastIdx[au.name] = au.s.cidx
				cidx = idx
			}
			ias := ApplicationsStatus{
				Applications: map[string]ApplicationStatus{au.name: au.s},
				TimeStamp:    au.s.TimeStamp,
				idx:          idx,
				cidx:         cidx,
			}
			go func(as ApplicationsStatus) { a.broker.notifier <- as }(ias)
		}
	}()
}

func (as *ApplicationsStatus) recalculate() {
	as.Degraded = false
	as.Failed = false
	as.ApplicationsTotal = 0
	as.ApplicationsUp = 0
	as.ApplicationsDegraded = 0
	as.ApplicationsFailed = 0
	as.ServicesTotal = 0
	as.ServicesUp = 0
	as.ServicesDegraded = 0
	as.ServicesFailed = 0
	as.InstancesTotal = 0
	as.InstancesUp = 0
	as.InstancesFailed = 0
	for _, a := range as.Applications {
		as.ApplicationsTotal++
		if !a.Failed && !a.Degraded {
			as.ApplicationsUp++
		}
		if a.Failed {
			as.Failed = true
			as.ApplicationsFailed++
		}
		if a.Degraded {
			as.Degraded = true
			as.ApplicationsDegraded++
		}
		as.ServicesTotal += a.ServicesTotal
		as.ServicesUp += a.ServicesUp
		as.ServicesDegraded += a.ServicesDegraded
		as.ServicesFailed += a.ServicesFailed
		as.InstancesTotal += a.InstancesTotal
		as.InstancesUp += a.InstancesUp
		as.InstancesFailed += a.InstancesFailed
	}
}

func (as *ApplicationsStatus) updateFrom(ias *ApplicationsStatus) { //nolint: dupl
	if ias.idx > as.idx {
		as.idx = ias.idx
	}
	if ias.cidx > as.cidx {
		as.cidx = ias.cidx
	}
	if as.Applications == nil {
		as.Applications = make(map[string]ApplicationStatus)
	}
	for iian, iias := range ias.Applications {
		aas := as.Applications[iian]
		aas.updateFrom(&iias)
		aas.recalculate()
		as.Applications[iian] = aas
		if as.TimeStamp.Before(aas.TimeStamp) {
			as.TimeStamp = iias.TimeStamp
		}
		if as.LastChange.Before(aas.LastChange) {
			as.LastChange = aas.LastChange
		}
	}
}

func (as *ApplicationsStatus) copySummaryFrom(ias *ApplicationsStatus) bool {
	if ias.idx > as.idx {
		as.idx = ias.idx
	}
	if ias.cidx > as.cidx {
		as.cidx = ias.cidx
	}
	if ias.TimeStamp.After(as.TimeStamp) {
		as.TimeStamp = ias.TimeStamp
	}
	c := as.Degraded == ias.Degraded && //nolint: dupl
		as.Failed == ias.Failed &&
		as.ApplicationsTotal == ias.ApplicationsTotal &&
		as.ApplicationsUp == ias.ApplicationsUp &&
		as.ApplicationsDegraded == ias.ApplicationsDegraded &&
		as.ApplicationsFailed == ias.ApplicationsFailed &&
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
	as.ApplicationsTotal = ias.ApplicationsTotal
	as.ApplicationsUp = ias.ApplicationsUp
	as.ApplicationsDegraded = ias.ApplicationsDegraded
	as.ApplicationsFailed = ias.ApplicationsFailed
	as.ServicesTotal = ias.ServicesTotal
	as.ServicesUp = ias.ServicesUp
	as.ServicesDegraded = ias.ServicesDegraded
	as.ServicesFailed = ias.ServicesFailed
	as.InstancesTotal = ias.InstancesTotal
	as.InstancesUp = ias.InstancesUp
	as.InstancesFailed = ias.InstancesFailed
	return true
}

func (as *ApplicationsStatus) contains(ias *ApplicationsStatus, depth uint8) bool {
	if depth <= 0 {
		return true
	}
	for an, a := range ias.Applications {
		asa, ok := as.Applications[an]
		if !ok {
			return false
		}
		if !asa.contains(&a, depth-1) {
			return false
		}
	}
	return true
}
