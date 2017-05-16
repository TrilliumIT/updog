package updog

type Application struct {
	Services map[string]*Service `json:"services"`
	Name     string
	state    ServiceState
}
