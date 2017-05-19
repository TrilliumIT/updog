package types

type Applications map[string]*Application

type ApplicationsStatus struct {
	Applications         map[string]*ApplicationStatus `json:"applications"`
	Degraded             bool                          `json:"degraded"`
	Failed               bool                          `json:"failed"`
	ApplicationsTotal    int                           `json:"applications_total"`
	ApplicationsUp       int                           `json:"applications_up"`
	ApplicationsDegraded int                           `json:"applications_degraded"`
	ApplicationsFailed   int                           `json:"applications_failed"`
	ServicesTotal        int                           `json:"services_total"`
	ServicesUp           int                           `json:"services_up"`
	ServicesDegraded     int                           `json:"services_degraded"`
	ServicesFailed       int                           `json:"services_failed"`
	InstancesTotal       int                           `json:"instances_total"`
	InstancesUp          int                           `json:"instances_up"`
	InstancesFailed      int                           `json:"instances_failed"`
}

func (apps Applications) GetApplicationsStatus() *ApplicationsStatus {
	type statusUpdate struct {
		an     string
		status *ApplicationStatus
	}
	rc := make(chan statusUpdate)
	defer close(rc)
	for an, app := range apps {
		go func(an string, app *Application) {
			rc <- statusUpdate{an: an, status: app.GetStatus()}
		}(an, app)
	}
	r := &ApplicationsStatus{Applications: make(map[string]*ApplicationStatus)}
	for range apps {
		a := <-rc
		r.Applications[a.an] = a.status
		r.ApplicationsTotal++
		if !a.status.Failed && !a.status.Degraded {
			r.ApplicationsUp++
		}
		if a.status.Failed {
			r.Failed = true
			r.ApplicationsFailed++
		}
		if a.status.Degraded {
			r.Degraded = true
			r.ApplicationsDegraded++
		}
		r.ServicesTotal += a.status.ServicesTotal
		r.ServicesUp += a.status.ServicesUp
		r.ServicesDegraded += a.status.ServicesDegraded
		r.ServicesFailed += a.status.ServicesFailed
		r.InstancesTotal += a.status.InstancesTotal
		r.InstancesUp += a.status.InstancesUp
		r.InstancesFailed += a.status.InstancesFailed
	}
	return r
}
