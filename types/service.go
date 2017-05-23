package types

import (
	log "github.com/sirupsen/logrus"
	"strings"
	"sync"
	"time"
)

type CheckOptions struct {
	Stype      string   `json:"type"`
	HttpMethod string   `json:"http_method"`
	Interval   Interval `json:"interval"`
}

type Service struct {
	Instances    []*Instance   `json:"instances"`
	MaxFailures  int           `json:"max_failures"`
	CheckOptions *CheckOptions `json:"check_options"`
	broker       *serviceBroker
	brokerLock   sync.Mutex
}

func (s *Service) GetStatus(depth uint8) ServiceStatus {
	sub := s.Subscribe(true, depth, 0)
	defer sub.Close()
	return <-sub.C
}

type ServiceSubscription struct {
	baseSubscription
	C       chan ServiceStatus
	close   chan chan ServiceStatus
	pending ServiceStatus
}

func (s *Service) Subscribe(full bool, depth uint8, maxStale time.Duration) *ServiceSubscription {
	if s.broker == nil {
		s.brokerLock.Lock()
		if s.broker == nil {
			s.broker = newServiceBroker()
		}
		s.brokerLock.Unlock()
	}
	r := &ServiceSubscription{
		C:     make(chan ServiceStatus),
		close: s.broker.closingClients,
		baseSubscription: baseSubscription{
			opts:     newBrokerOptions(full, depth).maxDepth(1),
			maxStale: maxStale,
		},
	}
	r.setMaxStale()
	s.broker.newClients <- r
	return r
}

func (s *Service) Sub(full bool, depth uint8, maxStale time.Duration) Subscription {
	return s.Subscribe(full, depth, maxStale)
}

func (s *ServiceSubscription) Close() {
	s.close <- s.C
}

func (s *ServiceSubscription) Next() interface{} {
	return <-s.C
}

type ServiceStatus struct {
	Instances       map[string]InstanceStatus `json:"instances"`
	AvgResponseTime time.Duration             `json:"average_response_time"`
	Degraded        bool                      `json:"degraded"`
	Failed          bool                      `json:"failed"`
	MaxFailures     int                       `json:"max_failures"`
	InstancesTotal  int                       `json:"instances_total"`
	InstancesUp     int                       `json:"instances_up"`
	InstancesFailed int                       `json:"instances_failed"`
	TimeStamp       time.Time                 `json:"timestamp"`
}

type serviceBroker struct {
	notifier       chan ServiceStatus
	newClients     chan *ServiceSubscription
	closingClients chan chan ServiceStatus
	clients        map[chan ServiceStatus]*ServiceSubscription
}

func newServiceBroker() *serviceBroker {
	b := &serviceBroker{
		notifier:       make(chan ServiceStatus),
		newClients:     make(chan *ServiceSubscription),
		closingClients: make(chan chan ServiceStatus),
		clients:        make(map[chan ServiceStatus]*ServiceSubscription),
	}
	go func() {
		var ss [4]ServiceStatus
		var updated [4]bool
		f := newBrokerOptions(true, 1)
		i := newBrokerOptions(false, 1)
		for {
			select {
			case c := <-b.newClients:
				b.clients[c.C] = c
				if !updated[f] {
					continue
				}
				r := newBrokerOptions(true, c.opts.depth())
				if !updated[r] {
					ss[r].filter(r, &ss[i], &ss[f])
					updated[r] = true
				}
				c.lastUpdate = ss[r].TimeStamp
				go func(c chan ServiceStatus, ss ServiceStatus) {
					c <- ss
				}(c.C, ss[r])
			case c := <-b.closingClients:
				delete(b.clients, c)
			case ss[i] = <-b.notifier:
				var changed [4]bool
				updated = [4]bool{}
				changed[f] = ss[f].filter(f, &ss[i], nil)
				updated[f] = true
				changed[i] = ss[i].filter(i, &ss[i], &ss[f])
				updated[i] = true
				// TODO return based on broker options
				for c, o := range b.clients {
					if !updated[o.opts] {
						changed[o.opts] = ss[o.opts].filter(o.opts, &ss[i], &ss[f])
						updated[o.opts] = true
					}
					if changed[o.opts] || ss[o.opts].TimeStamp.Sub(o.lastUpdate) >= o.maxStale {
						o.lastUpdate = ss[o.opts].TimeStamp
						go func(c chan ServiceStatus, ss ServiceStatus) {
							c <- ss
						}(c, ss[o.opts])
					}
				}
			}
		}
	}()
	return b
}

func (ss *ServiceStatus) filter(o brokerOptions, ssi, ssf *ServiceStatus) bool {
	changes := !ss.contains(ssi)
	log.WithFields(log.Fields{
		"ss":      ss,
		"ssi":     ssi,
		"ssf":     ssf,
		"o":       o,
		"changes": changes,
		"full":    o.full(),
		"depth":   o.depth(),
	}).Info("WTF")

	if changes {
		ss.updateInstancesFrom(ssi)
	}

	if !o.full() || o.depth() < 1 {
		changes = ss.copySummaryFrom(ssf)
	}

	if o.depth() >= 1 {
		return changes
	}

	ss.Instances = map[string]InstanceStatus{}
	return changes
}

func (s *Service) StartChecks() {
	if s.CheckOptions == nil {
		s.CheckOptions = &CheckOptions{}
	}

	if s.CheckOptions.Interval == 0 {
		s.CheckOptions.Interval = Interval(10 * time.Second)
	}

	if s.broker == nil {
		s.brokerLock.Lock()
		if s.broker == nil {
			s.broker = newServiceBroker()
		}
		s.brokerLock.Unlock()
	}

	type instanceStatusUpdate struct {
		name string
		s    InstanceStatus
	}
	updates := make(chan *instanceStatusUpdate)
	for _, i := range s.Instances {
		if s.CheckOptions.Stype == "" {
			s.CheckOptions.Stype = "tcp_connect"
			if strings.HasPrefix("http", i.address) {
				s.CheckOptions.Stype = "http_status"
				if s.CheckOptions.HttpMethod == "" {
					s.CheckOptions.HttpMethod = "GET"
				}
			}
		}
		i.StartChecks(s.CheckOptions)
		go func(i *Instance) {
			iSub := i.Subscribe()
			defer iSub.Close()
			for is := range iSub.C {
				updates <- &instanceStatusUpdate{s: is, name: i.address}
			}
		}(i)
	}

	go func() {
		for isu := range updates {
			l := log.WithField("name", isu.name).WithField("status", isu.s)
			l.Debug("Recieved status update")
			iss := ServiceStatus{
				Instances:   map[string]InstanceStatus{isu.name: isu.s},
				MaxFailures: s.MaxFailures,
			}
			go func(iss ServiceStatus) { s.broker.notifier <- iss }(iss)
		}
	}()
}

func (ss *ServiceStatus) recalculate() {
	ss.InstancesTotal = 0
	ss.InstancesFailed = 0
	ss.InstancesUp = 0
	ss.AvgResponseTime = time.Duration(0)
	ss.Degraded = false
	ss.Failed = false
	for _, is := range ss.Instances {
		ss.InstancesTotal++
		if is.Up {
			ss.InstancesUp++
		} else {
			ss.InstancesFailed++
			ss.Degraded = true
		}
		ss.AvgResponseTime += is.ResponseTime
	}
	if ss.InstancesTotal > 0 {
		ss.AvgResponseTime = ss.AvgResponseTime / time.Duration(ss.InstancesTotal)
	}
	ss.Failed = ss.InstancesFailed > ss.MaxFailures
	return
}

func (ss *ServiceStatus) updateInstancesFrom(iss *ServiceStatus) {
	ss.MaxFailures = iss.MaxFailures
	if ss.Instances == nil {
		ss.Instances = make(map[string]InstanceStatus)
	}
	for in, i := range iss.Instances {
		ss.Instances[in] = i
		if ss.TimeStamp.Before(i.TimeStamp) {
			ss.TimeStamp = i.TimeStamp
		}
	}
	ss.recalculate()
}

func (iss *ServiceStatus) copySummaryFrom(ss *ServiceStatus) bool {
	c := iss.InstancesTotal == ss.InstancesTotal &&
		iss.InstancesFailed == ss.InstancesFailed &&
		iss.InstancesUp == ss.InstancesUp &&
		iss.AvgResponseTime == ss.AvgResponseTime &&
		iss.Degraded == ss.Degraded &&
		iss.Failed == ss.Failed

	iss.TimeStamp = ss.TimeStamp
	if c {
		return false
	}

	iss.InstancesTotal = ss.InstancesTotal
	iss.InstancesFailed = ss.InstancesFailed
	iss.InstancesUp = ss.InstancesUp
	iss.AvgResponseTime = ss.AvgResponseTime
	iss.Degraded = ss.Degraded
	iss.Failed = ss.Failed
	return true
}

func (ss *ServiceStatus) contains(iss *ServiceStatus) bool {
	for in, i := range iss.Instances {
		ssi, ok := ss.Instances[in]
		if !ok {
			return false
		}
		if i.TimeStamp != ssi.TimeStamp {
			return false
		}
		if i.ResponseTime != ssi.ResponseTime {
			return false
		}
		if i.Up != ssi.Up {
			return false
		}
	}
	return true
}
