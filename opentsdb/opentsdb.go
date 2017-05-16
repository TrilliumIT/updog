package opentsdb

import (
	"bosun.org/collect"
	"bosun.org/opentsdb"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	opentsdbAddr string
	updateChan   chan *opentsdb.DataPoint
	tags         map[string]string
	blockTagSet  bool
	tagLock      sync.Mutex
}

func NewClient(host string, tags map[string]string) *Client {
	b := &Client{
		opentsdbAddr: host,
		updateChan:   make(chan *opentsdb.DataPoint),
		tags:         tags,
	}
	if b.opentsdbAddr == "" {
		return b
	}
	go func() {
		t := time.NewTicker(3 * time.Second)
		dps := []*opentsdb.DataPoint{}
		for {
			select {
			case dp := <-b.updateChan:
				dps = append(dps, dp)
				if len(dps) < 256 {
					continue
				}
			case <-t.C:
				if len(dps) < 0 {
					continue
				}
			}
			err := sendDataPoints(dps, b.opentsdbAddr)
			if err != nil {
				log.WithError(err).Error("Error sending data to opentsdb")
				continue
			}
			dps = []*opentsdb.DataPoint{}
		}
	}()
	return b
}

func sendDataPoints(dps []*opentsdb.DataPoint, addr string) error {
	resp, err := collect.SendDataPoints(dps, addr)
	if err == nil {
		defer resp.Body.Close()
		defer io.Copy(ioutil.Discard, resp.Body)
	}
	if err != nil {
		return err
	}
	// Some problem with connecting to the server; retry later.
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Bad status from opentsdb: %v", resp.StatusCode)
	}
	return nil
}

// NewClient creates a derivative client using different tags but the same host
func (b *Client) NewClient(tags map[string]string) *Client {
	nb := &Client{
		opentsdbAddr: b.opentsdbAddr,
		updateChan:   b.updateChan,
	}
	for k, v := range b.tags {
		nb.tags[k] = v
	}
	for k, v := range tags {
		nb.tags[k] = v
	}
	return nb
}

func (b *Client) Submit(metric string, value interface{}, timestamp time.Time) {
	if b.opentsdbAddr == "" {
		return
	}
	go func() {
		b.updateChan <- &opentsdb.DataPoint{
			Metric:    metric,
			Timestamp: timestamp.Unix(),
			Value:     value,
			Tags:      b.tags,
		}
	}()
}