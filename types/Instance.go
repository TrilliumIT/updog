package updog

import "time"

type Instance struct {
	address      string
	Up           bool
	ResponseTime time.Duration
}
