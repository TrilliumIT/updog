package updog

import "time"

type Instance struct {
	address      string
	up           bool
	responseTime time.Duration
}
