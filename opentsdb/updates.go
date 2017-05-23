package opentsdb

import (
	updog "github.com/TrilliumIT/updog/types"
)

func (c *Client) Subscribe(conf *updog.Config) error {
	for an, app := range conf.Applications.Applications {
		ac := c.NewClient(map[string]string{"application": an})
		for sn, svc := range app.Services {
			sc := ac.NewClient(map[string]string{"service": sn})
			go func(svc *updog.Service, sc *Client) {
				sub := svc.Subscribe(false, 255, 0, false)
				for ss := range sub.C {
					sc.Submit("updog.service.degraded", ss.Degraded, ss.TimeStamp)
					sc.Submit("updog.service.failed", ss.Failed, ss.TimeStamp)
					for addr, inst := range ss.Instances {
						ic := sc.NewClient(map[string]string{"instance": addr})
						ic.Submit("updog.instance.up", inst.Up, inst.TimeStamp)
						ic.Submit("updog.instance.response_time", inst.ResponseTime, inst.TimeStamp)
					}
				}
			}(svc, sc)
		}
	}
	return nil
}
