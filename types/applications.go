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
	sub := a.Subscribe(true)
	defer sub.Close()
	return <-sub.C
}

type ApplicationsSubscription struct {
	C     chan ApplicationsStatus
	close chan chan ApplicationsStatus
}

func (a *Applications) Subscribe(full bool) *ApplicationsSubscription {
	if a.broker == nil {
		a.broker = newApplicationsBroker()
		a.startSubscriptions()
	}
	r := &ApplicationsSubscription{C: make(chan ApplicationsStatus), close: a.broker.closingClients}
	if full {
		a.broker.newFullClients <- r.C
	} else {
		a.broker.newClients <- r.C
	}
	return r
}

func (a *Applications) Sub(full bool) Subscription {
	return a.Subscribe(full)
}

func (a *ApplicationsSubscription) Close() {
	a.close <- a.C
}

func (a *ApplicationsSubscription) Next() interface{} {
	return <-a.C
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
	newFullClients chan chan ApplicationsStatus
	closingClients chan chan ApplicationsStatus
	clients        map[chan ApplicationsStatus]struct{}
	fullClients    map[chan ApplicationsStatus]struct{}
}

func newApplicationsBroker() *applicationsBroker {
	b := &applicationsBroker{
		notifier:       make(chan ApplicationsStatus),
		newClients:     make(chan chan ApplicationsStatus),
		newFullClients: make(chan chan ApplicationsStatus),
		closingClients: make(chan chan ApplicationsStatus),
		clients:        make(map[chan ApplicationsStatus]struct{}),
		fullClients:    make(map[chan ApplicationsStatus]struct{}),
	}
	go func() {
		as := ApplicationsStatus{Applications: make(map[string]ApplicationStatus)}
		for {
			select {
			case c := <-b.newClients:
				b.clients[c] = struct{}{}
				go func(c chan ApplicationsStatus, as ApplicationsStatus) {
					c <- as
				}(c, as)
			case c := <-b.newFullClients:
				b.fullClients[c] = struct{}{}
				go func(c chan ApplicationsStatus, as ApplicationsStatus) {
					c <- as
				}(c, as)
			case c := <-b.closingClients:
				delete(b.clients, c)
				delete(b.fullClients, c)
			case ias := <-b.notifier:
				as = as.updateApplicationsFrom(&ias)
				ias = ias.copySummaryFrom(&as)
				for c := range b.clients {
					go func(c chan ApplicationsStatus, as ApplicationsStatus) {
						c <- ias
					}(c, ias)
				}
				for c := range b.fullClients {
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
			sub := a.Subscribe(false)
			defer sub.Close()
			for as := range sub.C {
				updates <- &applicationStatusUpdate{name: an, s: as}
			}
		}(an, a)
	}
	go func() {
		for au := range updates {
			ias := ApplicationsStatus{Applications: make(map[string]ApplicationStatus)}
			ias.Applications[au.name] = au.s
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

func (as ApplicationsStatus) updateApplicationsFrom(ias *ApplicationsStatus) ApplicationsStatus {
	for iian, iias := range ias.Applications {
		as.Applications[iian] = as.Applications[iian].updateServicesFrom(&iias)
	}
	as.recalculate()
	return as
}

func (ias ApplicationsStatus) copySummaryFrom(as *ApplicationsStatus) ApplicationsStatus {
	ias.Degraded = as.Degraded
	ias.Failed = as.Failed
	ias.ApplicationsTotal = as.ApplicationsTotal
	ias.ApplicationsUp = as.ApplicationsUp
	ias.ApplicationsDegraded = as.ApplicationsDegraded
	ias.ApplicationsFailed = as.ApplicationsFailed
	ias.ServicesTotal = as.ServicesTotal
	ias.ServicesUp = as.ServicesUp
	ias.ServicesDegraded = as.ServicesDegraded
	ias.ServicesFailed = as.ServicesFailed
	ias.InstancesTotal = as.InstancesTotal
	ias.InstancesUp = as.InstancesUp
	ias.InstancesFailed = as.InstancesFailed
	return ias
}
