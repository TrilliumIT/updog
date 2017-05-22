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
	sub := s.Subscribe(true, depth)
	defer sub.Close()
	return <-sub.C
}

type ServiceSubscription struct {
	C     chan ServiceStatus
	close chan chan ServiceStatus
	opts  brokerOptions
}

func (s *Service) Subscribe(full bool, depth uint8) *ServiceSubscription {
	if s.broker == nil {
		s.brokerLock.Lock()
		if s.broker == nil {
			s.broker = newServiceBroker()
		}
		s.brokerLock.Unlock()
	}
	r := &ServiceSubscription{C: make(chan ServiceStatus), close: s.broker.closingClients, opts: newBrokerOptions(full, depth).maxDepth(1)}
	s.broker.newClients <- r
	return r
}

func (s *Service) Sub(full bool, depth uint8) Subscription {
	return s.Subscribe(full, depth)
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
	clients        map[chan ServiceStatus]brokerOptions
}

func newServiceBroker() *serviceBroker {
	b := &serviceBroker{
		notifier:       make(chan ServiceStatus),
		newClients:     make(chan *ServiceSubscription),
		closingClients: make(chan chan ServiceStatus),
		clients:        make(map[chan ServiceStatus]brokerOptions),
	}
	go func() {
		var ss [3]ServiceStatus
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
					ss[c.opts], _ = ss[c.opts].filter(c.opts, &ss[f])
				}
				go func(c chan ServiceStatus, ss ServiceStatus) {
					c <- ss
				}(c.C, ss[c.opts])
			case c := <-b.closingClients:
				delete(b.clients, c)
			case ss[i] = <-b.notifier:
				ss[f] = ss[f].updateInstancesFrom(&ss[i])
				ss[i], _ = ss[i].copySummaryFrom(&ss[f])
				changed := brokerOptions(i | f)
				updated = brokerOptions(i | f)
				// TODO return based on broker options
				for c, o := range b.clients {
					if updated&o == 0 {
						var och brokerOptions
						ss[o], och = ss[o].filter(o, &ss[f])
						changed = changed | och
					}
					if changed&o == 1 {
						go func(c chan ServiceStatus, ss ServiceStatus) {
							c <- ss
						}(c, ss[o])
					}
				}
			}
		}
	}()
	return b
}

func (ss ServiceStatus) filter(o brokerOptions, ssf *ServiceStatus) (ServiceStatus, brokerOptions) {
	if o.depth() >= 1 {
		return *ssf, o
	}
	var changed bool
	ss, changed = ss.copySummaryFrom(ssf)
	ss.Instances = map[string]InstanceStatus{}
	if changed {
		return ss, o
	}
	return ss, 0
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
				TimeStamp:   isu.s.TimeStamp,
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

func (ss ServiceStatus) updateInstancesFrom(iss *ServiceStatus) ServiceStatus {
	ss.MaxFailures = iss.MaxFailures
	ss.TimeStamp = iss.TimeStamp
	if ss.Instances == nil {
		ss.Instances = make(map[string]InstanceStatus)
	}
	for in, i := range iss.Instances {
		ss.Instances[in] = i
	}
	ss.recalculate()
	return ss
}

func (iss ServiceStatus) copySummaryFrom(ss *ServiceStatus) (ServiceStatus, bool) {
	c := ss.TimeStamp.Sub(iss.TimeStamp) <= maxUpdate &&
		iss.InstancesTotal == ss.InstancesTotal &&
		iss.InstancesFailed == ss.InstancesFailed &&
		iss.InstancesUp == ss.InstancesUp &&
		iss.AvgResponseTime == ss.AvgResponseTime &&
		iss.Degraded == ss.Degraded &&
		iss.Failed == ss.Failed

	iss.TimeStamp = ss.TimeStamp
	if c {
		return iss, false
	}

	iss.InstancesTotal = ss.InstancesTotal
	iss.InstancesFailed = ss.InstancesFailed
	iss.InstancesUp = ss.InstancesUp
	iss.AvgResponseTime = ss.AvgResponseTime
	iss.Degraded = ss.Degraded
	iss.Failed = ss.Failed
	return iss, true
}
