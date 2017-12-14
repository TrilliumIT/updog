package types

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"
)

const maxInstanceDepth = 0

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Instance struct {
	address    string
	broker     *instanceBroker
	brokerLock sync.Mutex
}

func (i *Instance) Address() string {
	return i.address
}

func (i *Instance) UnmarshalJSON(data []byte) (err error) {
	return json.Unmarshal(data, &i.address)
}

func (i Instance) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.address)
}

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
		time.Sleep(time.Duration(rand.Int63n(interval.Nanoseconds())))
		t := time.NewTicker(interval)
		var up, lastUp bool
		var idx, cidx uint64
		var start, end time.Time
		for {
			idx++
			start = time.Now()
			switch co.Stype {
			case "tcp_connect":
				up = tcpConnectCheck(i.address, interval)
			case "http_status":
				up = httpStatusCheck(co.HttpOpts, i.address, interval)
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
			<-t.C
		}
	}()
}

func tcpConnectCheck(address string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err == nil {
		defer conn.Close()
	}
	return err == nil
}

func httpStatusCheck(opts *HttpOpts, address string, timeout time.Duration) bool {
	l := log.WithFields(log.Fields{"method": opts.HttpMethod, "address": address, "timeout": timeout, "skip_tls_verify": opts.SkipTLSVerify})
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

	client := http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	req, err := http.NewRequest(opts.HttpMethod, address, nil)
	if err != nil {
		l.WithError(err).Error("Failed to create http request.")
	}
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		defer io.Copy(ioutil.Discard, resp.Body)
	}
	if err != nil {
		l.WithError(err).Error("Error doing http request")
	}
	return err == nil && resp.StatusCode >= 200 && resp.StatusCode <= 399
}
