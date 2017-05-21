package types

type Subscriber interface {
	Sub(bool) Subscription
}

type Subscription interface {
	Next() interface{}
	Close()
}
