package opentsdb

import (
	"bosun.org/collect"
	"bosun.org/opentsdb"
	"bytes"
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
	for k, v := range b.tags {
		b.tags[k] = opentsdb.MustReplace(v, "_")
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
				if len(dps) <= 0 {
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
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		log.WithFields(log.Fields{
			"status code": resp.StatusCode,
			"body":        buf.String(),
		}).Error("Bad status from opentsdb")
		return fmt.Errorf("Bad status from opentsdb")
	}
	return nil
}

// NewClient creates a derivative client using different tags but the same host
func (b *Client) NewClient(tags map[string]string) *Client {
	nb := &Client{
		opentsdbAddr: b.opentsdbAddr,
		updateChan:   b.updateChan,
		tags:         make(map[string]string),
	}
	for k, v := range b.tags {
		nb.tags[k] = v
	}
	for k, v := range tags {
		nb.tags[k] = opentsdb.MustReplace(v, "_")
	}
	return nb
}

func (b *Client) Submit(metric string, value interface{}, timestamp time.Time) {
	if b.opentsdbAddr == "" {
		return
	}
	dp := &opentsdb.DataPoint{
		Metric:    metric,
		Timestamp: timestamp.Unix(),
		Value:     value,
		Tags:      b.tags,
	}
	switch v := dp.Value.(type) {
	case bool:
		if v {
			dp.Value = int(1)
		} else {
			dp.Value = int(0)
		}
	case time.Duration:
		dp.Value = v.Seconds() * 1e3
	}
	dp.Metric = opentsdb.MustReplace(dp.Metric, "_")
	go func() {
		b.updateChan <- dp
	}()
}
