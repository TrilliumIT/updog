package updog

type Application struct {
	Services map[string]*Service `json:"services"`
	Name     string
	state    ServiceState
}

type ApplicationStatus struct {
	Services map[string]struct {
		Instances map[string]Status
	}
}

func (app *Application) GetStatus() *ApplicationStatus {
	type statusUpdate struct {
		sn     string
		status map[string]Status
	}
	rc := make(chan statusUpdate)
	defer close(rc)
	for sn, s := range app.Services {
		go func(sn string, s *Service) {
			rc <- statusUpdate{sn: sn, status: s.GetInstances()}
		}(sn, s)
	}
	r := &ApplicationStatus{Services: make(map[string]struct{ Instances map[string]Status })}
	for range app.Services {
		s := <-rc
		r.Services[s.sn] = struct{ Instances map[string]Status }{Instances: make(map[string]Status)}
		for in, i := range s.status {
			r.Services[s.sn].Instances[in] = i
		}
	}
	return r
}
