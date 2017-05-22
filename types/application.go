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

func (a *Application) GetStatus(depth uint8) ApplicationStatus {
	sub := a.Subscribe(true, depth)
	defer sub.Close()
	return <-sub.C
}

type ApplicationSubscription struct {
	C     chan ApplicationStatus
	close chan chan ApplicationStatus
	opts  brokerOptions
}

func (a *Application) Subscribe(full bool, depth uint8) *ApplicationSubscription {
	if a.broker == nil {
		a.brokerLock.Lock()
		if a.broker == nil {
			a.broker = newApplicationBroker()
		}
		a.brokerLock.Unlock()
	}
	r := &ApplicationSubscription{C: make(chan ApplicationStatus), close: a.broker.closingClients, opts: newBrokerOptions(full, depth).maxDepth(2)}
	a.broker.newClients <- r
	return r
}

func (a *Application) Sub(full bool, depth uint8) Subscription {
	return a.Subscribe(full, depth)
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
}

type applicationBroker struct {
	notifier       chan ApplicationStatus
	newClients     chan *ApplicationSubscription
	closingClients chan chan ApplicationStatus
	clients        map[chan ApplicationStatus]brokerOptions
}

func newApplicationBroker() *applicationBroker {
	b := &applicationBroker{
		notifier:       make(chan ApplicationStatus),
		newClients:     make(chan *ApplicationSubscription),
		closingClients: make(chan chan ApplicationStatus),
		clients:        make(map[chan ApplicationStatus]brokerOptions),
	}
	go func() {
		var as [5]ApplicationStatus
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
				go func(c chan ApplicationStatus, as ApplicationStatus) {
					c <- as
				}(c.C, as[c.opts])
			case c := <-b.closingClients:
				delete(b.clients, c)
			case as[i] = <-b.notifier:
				as[f] = as[f].updateServicesFrom(&as[i])
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
						go func(c chan ApplicationStatus, as ApplicationStatus) {
							c <- as
						}(c, as[o])
					}
				}
			}
		}
	}()
	return b
}

func (as ApplicationStatus) filter(o brokerOptions, asf *ApplicationStatus) (ApplicationStatus, brokerOptions) {
	if o.depth() >= 2 {
		return *asf, o
	}

	var changed bool
	as, changed = as.copySummaryFrom(asf)
	var oc brokerOptions
	if changed {
		oc = o
	}

	if o.depth() == 1 {
		for _, s := range as.Services {
			s.Instances = map[string]InstanceStatus{}
		}
		return as, oc
	}

	as.Services = map[string]ServiceStatus{}
	return as, oc

}

func (a *Application) startSubscriptions() {
	type serviceStatusUpdate struct {
		name string
		s    ServiceStatus
	}
	updates := make(chan *serviceStatusUpdate)
	for sn, s := range a.Services {
		go func(sn string, s *Service) {
			sub := s.Subscribe(false, 255)
			defer sub.Close()
			for ss := range sub.C {
				updates <- &serviceStatusUpdate{name: sn, s: ss}
			}
		}(sn, s)
	}
	go func() {
		for su := range updates {
			as := ApplicationStatus{
				Services:  map[string]ServiceStatus{su.name: su.s},
				TimeStamp: su.s.TimeStamp,
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

func (as ApplicationStatus) updateServicesFrom(ias *ApplicationStatus) ApplicationStatus {
	as.TimeStamp = ias.TimeStamp
	if as.Services == nil {
		as.Services = make(map[string]ServiceStatus)
	}
	for isn, iss := range ias.Services {
		as.Services[isn] = as.Services[isn].updateInstancesFrom(&iss)
	}
	as.recalculate()
	return as
}

func (ias ApplicationStatus) copySummaryFrom(as *ApplicationStatus) (ApplicationStatus, bool) {
	c := as.TimeStamp.Sub(ias.TimeStamp) <= maxUpdate &&
		ias.Degraded == as.Degraded &&
		ias.Failed == as.Failed &&
		ias.ServicesTotal == as.ServicesTotal &&
		ias.ServicesUp == as.ServicesUp &&
		ias.ServicesDegraded == as.ServicesDegraded &&
		ias.ServicesFailed == as.ServicesFailed &&
		ias.InstancesTotal == as.InstancesTotal &&
		ias.InstancesUp == as.InstancesUp &&
		ias.InstancesFailed == as.InstancesFailed

	ias.TimeStamp = as.TimeStamp
	if c {
		return ias, false
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
	return ias, true
}
