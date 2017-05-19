package types

type Application struct {
	Services map[string]*Service `json:"services"`
	Name     string
}

type ApplicationStatus struct {
	Services         map[string]*ServiceStatus `json:"services"`
	Degraded         bool                      `json:"degraded"`
	Failed           bool                      `json:"failed"`
	ServicesTotal    int                       `json:"services_total"`
	ServicesUp       int                       `json:"services_up"`
	ServicesDegraded int                       `json:"services_degraded"`
	ServicesFailed   int                       `json:"services_failed"`
	InstancesTotal   int                       `json:"instances_total"`
	InstancesUp      int                       `json:"instances_up"`
	InstancesFailed  int                       `json:"instances_failed"`
}

func (app *Application) GetStatus() *ApplicationStatus {
	type statusUpdate struct {
		sn     string
		status *ServiceStatus
	}
	rc := make(chan statusUpdate)
	defer close(rc)
	for sn, s := range app.Services {
		go func(sn string, s *Service) {
			rc <- statusUpdate{sn: sn, status: s.GetStatus()}
		}(sn, s)
	}
	r := &ApplicationStatus{Services: make(map[string]*ServiceStatus)}
	for range app.Services {
		s := <-rc
		r.Services[s.sn] = s.status
		r.ServicesTotal++
		if !s.status.Failed && !s.status.Degraded {
			r.ServicesUp++
		}
		if s.status.Failed {
			r.Failed = true
			r.ServicesFailed++
		}
		if s.status.Degraded {
			r.Degraded = true
			r.ServicesDegraded++
		}
		r.InstancesTotal += s.status.InstancesTotal
		r.InstancesUp += s.status.InstancesUp
		r.InstancesFailed += s.status.InstancesFailed
	}
	return r
}
