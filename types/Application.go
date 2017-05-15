package updog

type ServiceState uint8

const (
	SERVICE_UP = iota
	SERVICE_DEGRADED
	SERVICE_DOWN
)

type Application struct {
	Services map[string]*Service `json:"services"`
	state    ServiceState
}
