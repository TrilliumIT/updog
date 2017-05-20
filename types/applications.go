package types

import (
	"encoding/json"
)

type Applications struct {
	Applications map[string]*Application
	broker       *applicationsBroker
}

func (a *Applications) UnmarshalJSON(data []byte) (err error) {
	return json.Unmarshal(data, &a.Applications)
}

func (a Applications) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.Applications)
}

func (a *Applications) GetStatus() ApplicationsStatus {
	sub := a.Subscribe()
	defer sub.Close()
	return <-sub.C
}

type ApplicationsSubscription struct {
	C     chan ApplicationsStatus
	close chan chan ApplicationsStatus
}

func (a *Applications) Subscribe() *ApplicationsSubscription {
	if a.broker == nil {
		a.broker = newApplicationsBroker()
		go a.startSubscriptions()
	}
	r := &ApplicationsSubscription{C: make(chan ApplicationsStatus), close: a.broker.closingClients}
	a.broker.newClients <- r.C
	return r
}

func (a *ApplicationsSubscription) Close() {
	a.close <- a.C
}

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
}

type applicationsBroker struct {
	notifier       chan ApplicationsStatus
	newClients     chan chan ApplicationsStatus
	closingClients chan chan ApplicationsStatus
	clients        map[chan ApplicationsStatus]struct{}
}

func newApplicationsBroker() *applicationsBroker {
	b := &applicationsBroker{
		notifier:       make(chan ApplicationsStatus),
		newClients:     make(chan chan ApplicationsStatus),
		closingClients: make(chan chan ApplicationsStatus),
		clients:        make(map[chan ApplicationsStatus]struct{}),
	}
	go func() {
		var as ApplicationsStatus
		for {
			select {
			case c := <-b.newClients:
				b.clients[c] = struct{}{}
				go func(c chan ApplicationsStatus, as ApplicationsStatus) {
					c <- as
				}(c, as)
			case c := <-b.closingClients:
				delete(b.clients, c)
			case as = <-b.notifier:
				for c := range b.clients {
					go func(c chan ApplicationsStatus, as ApplicationsStatus) {
						c <- as
					}(c, as)
				}
			}
		}
	}()
	return b
}

func (a *Applications) startSubscriptions() {
	type applicationStatusUpdate struct {
		name string
		s    ApplicationStatus
	}
	updates := make(chan *applicationStatusUpdate)
	for an, a := range a.Applications {
		go func(an string, a *Application) {
			sub := a.Subscribe()
			defer sub.Close()
			for as := range sub.C {
				updates <- &applicationStatusUpdate{name: an, s: as}
			}
		}(an, a)
	}
	as := ApplicationsStatus{Applications: make(map[string]ApplicationStatus)}
	for au := range updates {
		as.Applications[au.name] = au.s
		as.recalculate()
		go func(as ApplicationsStatus) { a.broker.notifier <- as }(as)
	}
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
