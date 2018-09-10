package opentsdb

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"bosun.org/collect"
	"bosun.org/opentsdb"
	"github.com/TrilliumIT/updog/utils"
	log "github.com/sirupsen/logrus"
)

//Client is an opentsdb client
type Client struct {
	opentsdbAddr string
	updateChan   chan *opentsdb.DataPoint
	tags         map[string]string
}

//NewClient creates a new opentsdb client
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
		defer utils.DiscardCloseBody(resp.Body)
	}
	if err != nil {
		return err
	}
	// Some problem with connecting to the server; retry later.
	if resp.StatusCode != http.StatusNoContent {
		l := log.WithFields(log.Fields{
			"status code": resp.StatusCode,
		})
		buf := new(bytes.Buffer)
		var b int64
		b, err = buf.ReadFrom(resp.Body)
		if err != nil {
			l.WithError(err).Error("Error reading body into buffer")
		}
		l = l.WithFields(log.Fields{
			"body": buf.String(),
		})
		l.WithField("bytes", b).Debug("read bytes")
		if resp.StatusCode == 400 && len(dps) > 1 { // bad datapoint, try each independently
			for _, d := range dps {
				err = sendDataPoints([]*opentsdb.DataPoint{d}, addr)
				if err != nil {
					log.WithError(err).WithField("d", d).Error("Bad opentsdb datapoint")
				}
			}
			return nil
		}
		l.Error("Bad status from opentsdb")
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

//Submit sumits an update
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
