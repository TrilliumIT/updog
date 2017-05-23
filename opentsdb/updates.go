package opentsdb

import (
	updog "github.com/TrilliumIT/updog/types"
)

func (c *Client) Subscribe(conf *updog.Config) error {
	go func() {
		sub := conf.Applications.Subscribe(false, 255, 0, false)
		for ass := range sub.C {
			ts := ass.TimeStamp
			c.Submit("updog.applications_total", ass.ApplicationsTotal, ts)
			c.Submit("updog.applications_up", ass.ApplicationsUp, ts)
			c.Submit("updog.applications_degraded", ass.ApplicationsDegraded, ts)
			c.Submit("updog.applications_failed", ass.ApplicationsFailed, ts)
			c.Submit("updog.services_total", ass.ServicesTotal, ts)
			c.Submit("updog.services_up", ass.ServicesUp, ts)
			c.Submit("updog.services_degraded", ass.ServicesDegraded, ts)
			c.Submit("updog.services_failed", ass.ServicesFailed, ts)
			c.Submit("updog.instances_total", ass.InstancesTotal, ts)
			c.Submit("updog.instances_up", ass.InstancesUp, ts)
			c.Submit("updog.instances_failed", ass.InstancesFailed, ts)
			for an, a := range ass.Applications {
				ac := c.NewClient(map[string]string{"application": an})
				ts := a.TimeStamp
				ac.Submit("updog.application.failed", a.Failed, ts)
				ac.Submit("updog.application.degraded", a.Degraded, ts)
				ac.Submit("updog.application.services_total", a.ServicesTotal, ts)
				ac.Submit("updog.application.services_up", a.ServicesUp, ts)
				ac.Submit("updog.application.services_degraded", a.ServicesDegraded, ts)
				ac.Submit("updog.application.services_failed", a.ServicesFailed, ts)
				ac.Submit("updog.application.instances_total", a.InstancesTotal, ts)
				ac.Submit("updog.application.instances_up", a.InstancesUp, ts)
				ac.Submit("updog.application.instances_failed", a.InstancesFailed, ts)
				for sn, s := range a.Services {
					sc := ac.NewClient(map[string]string{"service": sn})
					ts := s.TimeStamp
					sc.Submit("updog.service.failed", s.Failed, ts)
					sc.Submit("updog.service.degraded", s.Degraded, ts)
					sc.Submit("updog.service.instances_total", s.InstancesTotal, ts)
					sc.Submit("updog.service.instances_up", s.InstancesUp, ts)
					sc.Submit("updog.service.instances_failed", s.InstancesFailed, ts)
					for in, i := range s.Instances {
						ic := sc.NewClient(map[string]string{"instance": in})
						ts := i.TimeStamp
						ic.Submit("updog.instance.up", i.Up, ts)
						ic.Submit("updog.instance.response_time", i.ResponseTime, ts)
					}
				}
			}
		}
	}()
	return nil
}
