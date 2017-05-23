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
	sub := a.Subscribe(true, depth, 0)
	defer sub.Close()
	return <-sub.C
}

type ApplicationsSubscription struct {
	baseSubscription
	C       chan ApplicationsStatus
	close   chan chan ApplicationsStatus
	pending ApplicationsStatus
}

func (a *Applications) Subscribe(full bool, depth uint8, maxStale time.Duration) *ApplicationsSubscription {
	if a.broker == nil {
		a.brokerLock.Lock()
		if a.broker == nil {
			a.broker = newApplicationsBroker()
			a.startSubscriptions()
		}
		a.brokerLock.Unlock()
	}
	r := &ApplicationsSubscription{
		C:     make(chan ApplicationsStatus),
		close: a.broker.closingClients,
		baseSubscription: baseSubscription{
			opts:     newBrokerOptions(full, depth).maxDepth(3),
			maxStale: maxStale,
		},
	}
	r.setMaxStale()
	a.broker.newClients <- r
	return r
}

func (a *Applications) Sub(full bool, depth uint8, maxStale time.Duration) Subscription {
	return a.Subscribe(full, depth, maxStale)
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
	clients        map[chan ApplicationsStatus]*ApplicationsSubscription
}

func newApplicationsBroker() *applicationsBroker {
	b := &applicationsBroker{
		notifier:       make(chan ApplicationsStatus),
		newClients:     make(chan *ApplicationsSubscription),
		closingClients: make(chan chan ApplicationsStatus),
		clients:        make(map[chan ApplicationsStatus]*ApplicationsSubscription),
	}
	go func() {
		var as [8]ApplicationsStatus
		var updated [8]bool
		f := newBrokerOptions(true, 3)
		i := newBrokerOptions(false, 3)
		for {
			select {
			case c := <-b.newClients:
				b.clients[c.C] = c
				if !updated[f] {
					continue
				}
				r := newBrokerOptions(true, c.opts.depth())
				if !updated[r] {
					as[r].update(r, &as[i], &as[f])
					updated[r] = true
				}
				c.lastUpdate = as[r].TimeStamp
				go func(c chan ApplicationsStatus, as ApplicationsStatus) {
					c <- as
				}(c.C, as[r])
			case c := <-b.closingClients:
				delete(b.clients, c)
			case as[i] = <-b.notifier:
				var changed [8]bool
				updated = [8]bool{}
				changed[f] = as[f].update(f, &as[i], nil)
				updated[f] = true
				changed[i] = as[i].update(i, &as[i], &as[f])
				updated[i] = true
				for c, o := range b.clients {
					if !updated[o.opts] {
						updated[o.opts] = true
						changed[o.opts] = as[o.opts].update(o.opts, &as[i], &as[f])
					}
					bo := o.opts
					if o.lastUpdate.IsZero() {
						bo = newBrokerOptions(true, bo.depth())
					}
					if changed[bo] || as[bo].TimeStamp.Sub(o.lastUpdate) >= o.maxStale {
						o.lastUpdate = as[bo].TimeStamp
						go func(c chan ApplicationsStatus, as ApplicationsStatus) {
							c <- as
						}(c, as[bo])
					}
				}
			}
		}
	}()
	return b
}

func (as *ApplicationsStatus) update(o brokerOptions, asi, asf *ApplicationsStatus) bool {

	changes := !as.contains(asi)
	if changes {
		as.updateApplicationsFrom(asi)
	}

	if o.full() && o.depth() >= 3 {
		as.recalculate()
		return changes
	}

	if o.full() && !as.contains(asf) {
		as.updateApplicationsFrom(asf)
	}

	changes = as.copySummaryFrom(asf)

	if o.depth() >= 3 {
		return changes
	}

	if o.depth() >= 2 {
		for an, a := range as.Applications {
			for sn, s := range as.Applications[an].Services {
				s.Instances = map[string]InstanceStatus{}
				a.Services[sn] = s
			}
			as.Applications[an] = a
		}
		return changes
	}

	if o.depth() >= 1 {
		for an, a := range as.Applications {
			a.Services = map[string]ServiceStatus{}
			as.Applications[an] = a
		}
		return changes
	}

	as.Applications = map[string]ApplicationStatus{}
	return changes

}

func (a *Applications) startSubscriptions() {
	type applicationStatusUpdate struct {
		name string
		s    ApplicationStatus
	}
	updates := make(chan *applicationStatusUpdate)
	for an, a := range a.Applications {
		go func(an string, a *Application) {
			sub := a.Subscribe(false, 255, 0)
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

func (as *ApplicationsStatus) updateApplicationsFrom(ias *ApplicationsStatus) {
	if as.Applications == nil {
		as.Applications = make(map[string]ApplicationStatus)
	}
	for iian, iias := range ias.Applications {
		aas := as.Applications[iian]
		aas.updateServicesFrom(&iias)
		as.Applications[iian] = aas
		if as.TimeStamp.Before(aas.TimeStamp) {
			as.TimeStamp = iias.TimeStamp
		}
	}
}

func (ias *ApplicationsStatus) copySummaryFrom(as *ApplicationsStatus) bool {
	if as.TimeStamp.After(ias.TimeStamp) {
		ias.TimeStamp = as.TimeStamp
	}
	c := ias.Degraded == as.Degraded &&
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

	if c {
		return false
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
	return true
}

func (as *ApplicationsStatus) contains(ias *ApplicationsStatus) bool {
	for an, a := range ias.Applications {
		asa, ok := as.Applications[an]
		if !ok {
			return false
		}
		if !asa.contains(&a) {
			return false
		}
	}
	return true
}
