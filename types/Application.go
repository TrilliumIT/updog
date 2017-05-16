package types

type Application struct {
	Services map[string]*Service `json:"services"`
	Name     string
}

type ApplicationStatus struct {
	Services map[string]ServiceStatus
}

func (app *Application) GetStatus() *ApplicationStatus {
	type statusUpdate struct {
		sn     string
		status ServiceStatus
	}
	rc := make(chan statusUpdate)
	defer close(rc)
	for sn, s := range app.Services {
		go func(sn string, s *Service) {
			rc <- statusUpdate{sn: sn, status: s.GetStatus()}
		}(sn, s)
	}
	r := &ApplicationStatus{Services: make(map[string]ServiceStatus)}
	for range app.Services {
		s := <-rc
		r.Services[s.sn] = s.status
	}
	return r
}
