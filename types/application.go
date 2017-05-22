package types

import "time"

type Application struct {
	Services map[string]*Service `json:"services"`
	broker   *applicationBroker
}

func (a *Application) GetStatus() ApplicationStatus {
	sub := a.Subscribe(true)
	defer sub.Close()
	return <-sub.C
}

type ApplicationSubscription struct {
	C     chan ApplicationStatus
	close chan chan ApplicationStatus
	opts  brokerOptions
}

func (a *Application) Subscribe(full bool) *ApplicationSubscription {
	if a.broker == nil {
		a.broker = newApplicationBroker()
		a.startSubscriptions()
	}
	r := &ApplicationSubscription{C: make(chan ApplicationStatus), close: a.broker.closingClients}
	if full {
		a.broker.newFullClients <- r.C
	} else {
		a.broker.newClients <- r.C
	}
	return r
}

func (a *Application) Sub(full bool) Subscription {
	return a.Subscribe(full)
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
	newClients     chan chan ApplicationStatus
	newFullClients chan chan ApplicationStatus
	closingClients chan chan ApplicationStatus
	clients        map[chan ApplicationStatus]struct{}
	fullClients    map[chan ApplicationStatus]struct{}
}

func newApplicationBroker() *applicationBroker {
	b := &applicationBroker{
		notifier:       make(chan ApplicationStatus),
		newClients:     make(chan chan ApplicationStatus),
		newFullClients: make(chan chan ApplicationStatus),
		closingClients: make(chan chan ApplicationStatus),
		clients:        make(map[chan ApplicationStatus]struct{}),
		fullClients:    make(map[chan ApplicationStatus]struct{}),
	}
	go func() {
		var as ApplicationStatus
		for {
			select {
			case c := <-b.newClients:
				b.clients[c] = struct{}{}
				go func(c chan ApplicationStatus, as ApplicationStatus) {
					c <- as
				}(c, as)
			case c := <-b.newFullClients:
				b.fullClients[c] = struct{}{}
				go func(c chan ApplicationStatus, as ApplicationStatus) {
					c <- as
				}(c, as)
			case c := <-b.closingClients:
				delete(b.clients, c)
				delete(b.fullClients, c)
			case ias := <-b.notifier:
				as = as.updateServicesFrom(&ias)
				ias = ias.copySummaryFrom(&as)
				for c := range b.fullClients {
					go func(c chan ApplicationStatus, as ApplicationStatus) {
						c <- as
					}(c, as)
				}
				for c := range b.clients {
					go func(c chan ApplicationStatus, as ApplicationStatus) {
						c <- ias
					}(c, ias)
				}
			}
		}
	}()
	return b
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

func (ias ApplicationStatus) copySummaryFrom(as *ApplicationStatus) ApplicationStatus {
	ias.Degraded = as.Degraded
	ias.Failed = as.Failed
	ias.ServicesTotal = as.ServicesTotal
	ias.ServicesUp = as.ServicesUp
	ias.ServicesDegraded = as.ServicesDegraded
	ias.ServicesFailed = as.ServicesFailed
	ias.InstancesTotal = as.InstancesTotal
	ias.InstancesUp = as.InstancesUp
	ias.InstancesFailed = as.InstancesFailed
	return ias
}
