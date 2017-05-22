package types

import (
	"encoding/json"
	"sync"
	"time"
)

type Applications struct {
	Applications map[string]*Application
	broker       *applicationsBroker
	brokerLock   sync.Mutex
}

func (a *Applications) UnmarshalJSON(data []byte) (err error) {
	return json.Unmarshal(data, &a.Applications)
}

func (a Applications) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.Applications)
}

func (a *Applications) GetStatus(depth uint8) ApplicationsStatus {
	sub := a.Subscribe(true, depth)
	defer sub.Close()
	return <-sub.C
}

type ApplicationsSubscription struct {
	C     chan ApplicationsStatus
	close chan chan ApplicationsStatus
	opts  brokerOptions
}

func (a *Applications) Subscribe(full bool, depth uint8) *ApplicationsSubscription {
	if a.broker == nil {
		a.brokerLock.Lock()
		if a.broker == nil {
			a.broker = newApplicationsBroker()
		}
		a.brokerLock.Unlock()
	}
	r := &ApplicationsSubscription{C: make(chan ApplicationsStatus), close: a.broker.closingClients, opts: newBrokerOptions(full, depth).maxDepth(3)}
	a.broker.newClients <- r
	return r
}

func (a *Applications) Sub(full bool, depth uint8) Subscription {
	return a.Subscribe(full, depth)
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
	TimeStamp            time.Time                    `json:"timestamp"`
}

type applicationsBroker struct {
	notifier       chan ApplicationsStatus
	newClients     chan *ApplicationsSubscription
	closingClients chan chan ApplicationsStatus
	clients        map[chan ApplicationsStatus]brokerOptions
}

func newApplicationsBroker() *applicationsBroker {
	b := &applicationsBroker{
		notifier:       make(chan ApplicationsStatus),
		newClients:     make(chan *ApplicationsSubscription),
		closingClients: make(chan chan ApplicationsStatus),
		clients:        make(map[chan ApplicationsStatus]brokerOptions),
	}
	go func() {
		var as [7]ApplicationsStatus
		f := newBrokerOptions(true, 1)
		i := newBrokerOptions(false, 1)
		var updated brokerOptions
		for {
			select {
			case c := <-b.newClients:
				b.clients[c.C] = c.opts
				if updated&f == 0 {
					continue
				}
				if updated&c.opts == 0 {
					as[c.opts], _ = as[c.opts].filter(c.opts, &as[f])
				}
				go func(c chan ApplicationsStatus, as ApplicationsStatus) {
					c <- as
				}(c.C, as[c.opts])
			case c := <-b.closingClients:
				delete(b.clients, c)
			case as[i] = <-b.notifier:
				as[f] = as[f].updateApplicationsFrom(&as[i])
				as[i], _ = as[i].copySummaryFrom(&as[f])
				changed := brokerOptions(i | f)
				updated = brokerOptions(i | f)
				for c, o := range b.clients {
					if updated&o == 0 {
						var och brokerOptions
						as[o], och = as[o].filter(o, &as[f])
						changed = changed | och
					}
					if changed&o == 1 {
						go func(c chan ApplicationsStatus, as ApplicationsStatus) {
							c <- as
						}(c, as[o])
					}
				}
			}
		}
	}()
	return b
}

func (as ApplicationsStatus) filter(o brokerOptions, asf *ApplicationsStatus) (ApplicationsStatus, brokerOptions) {
	if o.depth() >= 3 {
		return *asf, o
	}

	var changed bool
	as, changed = as.copySummaryFrom(asf)
	var oc brokerOptions
	if changed {
		oc = o
	}

	if o.depth() == 2 {
		for _, a := range as.Applications {
			for _, s := range a.Services {
				s.Instances = map[string]InstanceStatus{}
			}
		}
		return as, o
	}

	if o.depth() == 1 {
		for _, a := range as.Applications {
			a.Services = map[string]ServiceStatus{}
		}
		return as, oc
	}

	as.Applications = map[string]ApplicationStatus{}
	return as, o

}

func (a *Applications) startSubscriptions() {
	type applicationStatusUpdate struct {
		name string
		s    ApplicationStatus
	}
	updates := make(chan *applicationStatusUpdate)
	for an, a := range a.Applications {
		go func(an string, a *Application) {
			sub := a.Subscribe(false, 255)
			defer sub.Close()
			for as := range sub.C {
				updates <- &applicationStatusUpdate{name: an, s: as}
			}
		}(an, a)
	}
	go func() {
		for au := range updates {
			ias := ApplicationsStatus{
				Applications: map[string]ApplicationStatus{au.name: au.s},
				TimeStamp:    au.s.TimeStamp,
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

func (as ApplicationsStatus) updateApplicationsFrom(ias *ApplicationsStatus) ApplicationsStatus {
	as.TimeStamp = ias.TimeStamp
	if as.Applications == nil {
		as.Applications = make(map[string]ApplicationStatus)
	}
	for iian, iias := range ias.Applications {
		as.Applications[iian] = as.Applications[iian].updateServicesFrom(&iias)
	}
	as.recalculate()
	return as
}

func (ias ApplicationsStatus) copySummaryFrom(as *ApplicationsStatus) (ApplicationsStatus, bool) {
	c := as.TimeStamp.Sub(ias.TimeStamp) <= maxUpdate &&
		ias.Degraded == as.Degraded &&
		ias.Failed == as.Failed &&
		ias.ApplicationsTotal == as.ApplicationsTotal &&
		ias.ApplicationsUp == as.ApplicationsUp &&
		ias.ApplicationsDegraded == as.ApplicationsDegraded &&
		ias.ApplicationsFailed == as.ApplicationsFailed &&
		ias.ServicesTotal == as.ServicesTotal &&
		ias.ServicesUp == as.ServicesUp &&
		ias.ServicesDegraded == as.ServicesDegraded &&
		ias.ServicesFailed == as.ServicesFailed &&
		ias.InstancesTotal == as.InstancesTotal &&
		ias.InstancesUp == as.InstancesUp &&
		ias.InstancesFailed == as.InstancesFailed &&
		ias.Degraded == as.Degraded &&
		ias.Failed == as.Failed

	ias.TimeStamp = as.TimeStamp
	if c {
		return ias, false
	}

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
	return ias, true
}
