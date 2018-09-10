package types

import "time"

//go:generate genify -in=subscriber.gen -out=gen-subscriber.go

type brokerOptions uint8

func newBrokerOptions(full bool, depth uint8) brokerOptions {
	var f uint8
	if full {
		f = 1
	}
	return brokerOptions(depth<<1 | f)
}

func (b brokerOptions) maxDepth(md uint8) brokerOptions {
	if b.depth() > md {
		return newBrokerOptions(b.full(), md)
	}
	return b
}

func (b brokerOptions) depth() uint8 {
	return uint8(b >> 1)
}

func (b brokerOptions) full() bool {
	return b&1 == 1
}

//Subscriber is a browser that wants to stream updates
type Subscriber interface {
	Sub(bool, uint8, time.Duration, bool) Subscription
}

//Subscription represents the browser streaming connection
type Subscription interface {
	Next() interface{}
	Close()
}

type baseSubscription struct {
	opts        brokerOptions
	onlyChanges bool
	maxStale    time.Duration
	lastUpdate  time.Time //nolint: structcheck
	lastIdx     uint64    //nolint: structcheck
}

func (s *baseSubscription) setMaxStale() {
	if s.maxStale == 0 {
		// You kept a subscription open for 10 years:
		// you're getting an update, like it or not
		s.maxStale = time.Hour * 24 * 3652
	}
}
