package types

type Subscriber interface {
	Sub() Subscription
}

type Subscription interface {
	Next() interface{}
	Close()
}
