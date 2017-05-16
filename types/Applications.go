package types

type Applications map[string]*Application

func (apps Applications) GetApplicationStatus() map[string]*ApplicationStatus {
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
	r := make(map[string]*ApplicationStatus)
	for range apps {
		a := <-rc
		r[a.an] = a.status
	}
	return r
}
