package types

import (
	"sync"
	"time"
)

type Application struct {
	Services   map[string]*Service `json:"services"`
	broker     *applicationBroker
	brokerLock sync.Mutex
}

type ApplicationSubscription struct {
	baseSubscription
	C       chan ApplicationStatus
	close   chan chan ApplicationStatus
	pending ApplicationStatus
}

func (a *Application) Subscribe(full bool, depth uint8, maxStale time.Duration, onlyChanges bool) *ApplicationSubscription {
	if a.broker == nil {
		a.brokerLock.Lock()
		if a.broker == nil {
			a.broker = newApplicationBroker()
			a.startSubscriptions()
		}
		a.brokerLock.Unlock()
	}
	r := &ApplicationSubscription{
		C:     make(chan ApplicationStatus),
		close: a.broker.closingClients,
		baseSubscription: baseSubscription{
			opts:        newBrokerOptions(full, depth).maxDepth(2),
			maxStale:    maxStale,
			onlyChanges: onlyChanges,
		},
	}
	r.setMaxStale()
	a.broker.newClients <- r
	return r
}

func (a *Application) Sub(full bool, depth uint8, maxStale time.Duration, onlyChanges bool) Subscription {
	return a.Subscribe(full, depth, maxStale, onlyChanges)
}

func (a *ApplicationSubscription) Close() {
	a.close <- a.C
}

func (a *ApplicationSubscription) Next() interface{} {
	return <-a.C
}

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

type applicationBroker struct {
	notifier       chan ApplicationStatus
	newClients     chan *ApplicationSubscription
	closingClients chan chan ApplicationStatus
	clients        map[chan ApplicationStatus]*ApplicationSubscription
}

func newApplicationBroker() *applicationBroker {
	b := &applicationBroker{
		notifier:       make(chan ApplicationStatus),
		newClients:     make(chan *ApplicationSubscription),
		closingClients: make(chan chan ApplicationStatus),
		clients:        make(map[chan ApplicationStatus]*ApplicationSubscription),
	}
	go func() {
		var as [6]ApplicationStatus
		f := newBrokerOptions(true, 2)
		i := newBrokerOptions(false, 2)
		var updated [6]bool
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
				go func(c chan ApplicationStatus, as ApplicationStatus) {
					c <- as
				}(c.C, as[r])
			case c := <-b.closingClients:
				delete(b.clients, c)
			case as[i] = <-b.notifier:
				var changed [6]bool
				updated = [6]bool{}
				as[f].updateServicesFrom(&as[i])
				as[f].recalculate()
				changed[f] = true
				updated[f] = true
				as[i].copySummaryFrom(&as[f])
				changed[i] = true
				updated[i] = true
				for c, o := range b.clients {
					if !updated[o.opts] {
						changed[o.opts] = as[o.opts].update(o.opts, &as[i], &as[f])
						updated[o.opts] = true
					}
					if changed[o.opts] || as[o.opts].TimeStamp.Sub(o.lastUpdate) >= o.maxStale {
						o.lastUpdate = as[o.opts].TimeStamp
						go func(c chan ApplicationStatus, as ApplicationStatus) {
							c <- as
						}(c, as[o.opts])
					}
				}
			}
		}
	}()
	return b
}

func (as *ApplicationStatus) update(o brokerOptions, asi, asf *ApplicationStatus) bool {
	changes := as.copySummaryFrom(asf)

	asu := asi
	if o.full() {
		asu = asf
	}

	if !as.contains(asu, o.depth()) {
		as.updateServicesFrom(asu)
		changes = true
	}

	if o.depth() <= 0 {
		as.Services = map[string]ServiceStatus{}
		return changes
	}

	if o.depth() == 1 {
		for sn, s := range as.Services {
			s.Instances = map[string]InstanceStatus{}
			as.Services[sn] = s
		}
		return changes
	}

	return changes
}

func (a *Application) startSubscriptions() {
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
	return
}

func (as *ApplicationStatus) updateServicesFrom(ias *ApplicationStatus) {
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
		ass.updateInstancesFrom(&iss)
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

func (ias *ApplicationStatus) copySummaryFrom(as *ApplicationStatus) bool {
	if as.idx > ias.idx {
		ias.idx = as.idx
	}
	if as.cidx > ias.cidx {
		ias.cidx = as.cidx
	}
	if ias.TimeStamp.Before(as.TimeStamp) {
		ias.TimeStamp = as.TimeStamp
	}
	c := ias.Degraded == as.Degraded &&
		ias.Failed == as.Failed &&
		ias.ServicesTotal == as.ServicesTotal &&
		ias.ServicesUp == as.ServicesUp &&
		ias.ServicesDegraded == as.ServicesDegraded &&
		ias.ServicesFailed == as.ServicesFailed &&
		ias.InstancesTotal == as.InstancesTotal &&
		ias.InstancesUp == as.InstancesUp &&
		ias.InstancesFailed == as.InstancesFailed

	if c {
		return false
	}

	ias.Degraded = as.Degraded
	ias.Failed = as.Failed
	ias.ServicesTotal = as.ServicesTotal
	ias.ServicesUp = as.ServicesUp
	ias.ServicesDegraded = as.ServicesDegraded
	ias.ServicesFailed = as.ServicesFailed
	ias.InstancesTotal = as.InstancesTotal
	ias.InstancesUp = as.InstancesUp
	ias.InstancesFailed = as.InstancesFailed
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
