package types

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/TrilliumIT/updog/utils"
	log "github.com/sirupsen/logrus"
)

const maxInstanceDepth = 0

func init() {
	rand.Seed(time.Now().UnixNano())
}

//Instance represents a single running daemon on a single host
type Instance struct {
	address    string
	broker     *instanceBroker
	brokerLock sync.Mutex
}

//Address returns the instance address
func (i *Instance) Address() string {
	return i.address
}

//UnmarshalJSON unmarshals the JSON bytes
func (i *Instance) UnmarshalJSON(data []byte) (err error) {
	return json.Unmarshal(data, &i.address)
}

//MarshalJSON marshals the data structure to a byte array
func (i *Instance) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.address)
}

//InstanceStatus represents the status of the instance
type InstanceStatus struct {
	Up           bool          `json:"up"`
	ResponseTime time.Duration `json:"response_time"`
	TimeStamp    time.Time     `json:"timestamp"`
	LastChange   time.Time     `json:"last_change"`
	idx, cidx    uint64
}

func newInstanceBroker() *instanceBroker {
	b := &instanceBroker{
		notifier:       make(chan InstanceStatus),
		newClients:     make(chan *InstanceSubscription),
		closingClients: make(chan chan InstanceStatus),
		clients:        make(map[chan InstanceStatus]*InstanceSubscription),
	}
	go func() {
		var is InstanceStatus
		for {
			select {
			case c := <-b.newClients:
				log.WithField("b", b).WithField("is", is).Debug("newClient")
				b.clients[c.C] = c
				if is.idx > 0 {
					go func(c chan InstanceStatus, is InstanceStatus) {
						c <- is
					}(c.C, is)
				}
			case c := <-b.closingClients:
				delete(b.clients, c)
			case is = <-b.notifier:
				log.WithField("b", b).WithField("is", is).Debug("Notified")
				for c, o := range b.clients {
					if !o.onlyChanges || o.lastIdx < is.cidx {
						go func(c chan InstanceStatus, is InstanceStatus) {
							c <- is
						}(c, is)
					}
				}
			}
		}
	}()
	return b
}

func (i *Instance) startSubscriptions() {
}

//StartChecks launches a go routine to monitor the instance
func (i *Instance) StartChecks(co *CheckOptions) {
	log.WithField("Address", i.address).Debug("Starting checks")

	if i.broker == nil {
		i.brokerLock.Lock()
		if i.broker == nil {
			i.broker = newInstanceBroker()
		}
		i.brokerLock.Unlock()
	}

	interval := time.Duration(co.Interval)
	go func() {
		var t *time.Ticker
		var up, lastUp bool
		var idx, cidx uint64
		var start, end time.Time
		var client *http.Client
		for {
			idx++
			start = time.Now()
			switch co.Stype {
			case TCPConnect:
				up = tcpConnectCheck(i.address, interval)
			case HTTPStatus:
				if client == nil {
					client = newHTTPClient(co.HTTPOpts, interval)
				}
				up = httpStatusCheck(co.HTTPOpts, i.address, client)
			default:
				log.WithField("type", co.Stype).Error("Unknown service type")
				return
			}
			end = time.Now()
			if up != lastUp {
				lastUp = up
				cidx = idx
			}
			go func(st InstanceStatus) { i.broker.notifier <- st }(InstanceStatus{
				Up:           up,
				ResponseTime: end.Sub(start),
				TimeStamp:    start,
				idx:          idx,
				cidx:         cidx,
			})
			// This allows the first check to run immediately, then create the ticker
			// then continue so we don't sleep random + ticker time
			if t == nil {
				time.Sleep(time.Duration(rand.Int63n(interval.Nanoseconds())))
				t = time.NewTicker(interval)
				continue
			}
			<-t.C
		}
	}()
}

func tcpConnectCheck(address string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err == nil {
		defer func() {
			err = conn.Close()
			if err != nil {
				log.WithError(err).Error("Error closing connection")
			}
		}()
	}
	return err == nil
}

func newHTTPClient(opts *HTTPOpts, timeout time.Duration) *http.Client {
	l := log.WithFields(log.Fields{"method": opts.HTTPMethod, "timeout": timeout, "skip_tls_verify": opts})
	tlsConfig := &tls.Config{
		InsecureSkipVerify: opts.SkipTLSVerify,
	}

	if opts.CA != "" {
		rootCert := x509.NewCertPool()
		data, err := ioutil.ReadFile(opts.CA)
		if err != nil {
			l.WithError(err).WithField("file", opts.CA).Error("Error reading CA file")
		}
		ok := rootCert.AppendCertsFromPEM(data)
		if !ok {
			l.Error("Error adding ca to root store")
		}
		tlsConfig.RootCAs = rootCert
	}

	if (opts.ClientCert != "") || (opts.ClientKey != "") {
		clientCert, err := tls.LoadX509KeyPair(opts.ClientCert, opts.ClientKey)
		if err != nil {
			l.WithField("certificate", opts.ClientCert).WithField("key", opts.ClientKey).WithError(err).Error("Unable to load client tls key")
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, clientCert)
	}

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return client
}

func httpStatusCheck(opts *HTTPOpts, address string, client *http.Client) bool {
	l := log.WithFields(log.Fields{"method": opts.HTTPMethod, "address": address, "skip_tls_verify": opts})
	req, err := http.NewRequest(opts.HTTPMethod, address, nil)
	if err != nil {
		l.WithError(err).Error("Failed to create http request.")
	}
	resp, err := client.Do(req)
	if err == nil {
		defer utils.DiscardCloseBody(resp.Body)
	}
	if err != nil {
		l.WithError(err).Error("Error doing http request")
	}
	return err == nil && resp.StatusCode >= 200 && resp.StatusCode <= 399
}
