package types

type Application struct {
	Services map[string]*Service `json:"services"`
	broker   *applicationBroker
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
}

type applicationBroker struct {
	notifier       chan ApplicationStatus
	newClients     chan chan ApplicationStatus
	closingClients chan chan ApplicationStatus
	clients        map[chan ApplicationStatus]struct{}
}

type ApplicationSubscription struct {
	C     chan ApplicationStatus
	close chan chan ApplicationStatus
}

func (a *Application) GetStatus() ApplicationStatus {
	sub := a.Subscribe()
	defer sub.Close()
	return <-sub.C
}

func (a *Application) Subscribe() *ApplicationSubscription {
	if a.broker == nil {
		a.broker = newApplicationBroker()
		go a.startSubscriptions()
	}
	r := &ApplicationSubscription{C: make(chan ApplicationStatus), close: a.broker.closingClients}
	a.broker.newClients <- r.C
	return r
}

func (a *ApplicationSubscription) Close() {
	a.close <- a.C
	close(a.C)
}

func newApplicationBroker() *applicationBroker {
	b := &applicationBroker{
		notifier:       make(chan ApplicationStatus),
		newClients:     make(chan chan ApplicationStatus),
		closingClients: make(chan chan ApplicationStatus),
		clients:        make(map[chan ApplicationStatus]struct{}),
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
			case c := <-b.closingClients:
				delete(b.clients, c)
			case as = <-b.notifier:
				for c := range b.clients {
					go func(c chan ApplicationStatus, as ApplicationStatus) {
						c <- as
					}(c, as)
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
			sub := s.Subscribe()
			defer sub.Close()
			for ss := range sub.C {
				updates <- &serviceStatusUpdate{name: sn, s: ss}
			}
		}(sn, s)
	}
	as := ApplicationStatus{}
	for su := range updates {
		as.Services[su.name] = su.s
		as.recalculate()
		go func(as ApplicationStatus) { a.broker.notifier <- as }(as)
	}
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
