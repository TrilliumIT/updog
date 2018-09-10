package opentsdb

import (
	updog "github.com/TrilliumIT/updog/types"
)

//Subscribe subscribes opentsdb client to the server to continuously send data
func (c *Client) Subscribe(conf *updog.Config) error {
	go func() {
		sub := conf.Applications.Subscribe(false, 255, 0, false)
		for ass := range sub.C {
			asts := ass.TimeStamp
			c.Submit("updog.applications_total", ass.ApplicationsTotal, asts)
			c.Submit("updog.applications_up", ass.ApplicationsUp, asts)
			c.Submit("updog.applications_degraded", ass.ApplicationsDegraded, asts)
			c.Submit("updog.applications_failed", ass.ApplicationsFailed, asts)
			c.Submit("updog.services_total", ass.ServicesTotal, asts)
			c.Submit("updog.services_up", ass.ServicesUp, asts)
			c.Submit("updog.services_degraded", ass.ServicesDegraded, asts)
			c.Submit("updog.services_failed", ass.ServicesFailed, asts)
			c.Submit("updog.instances_total", ass.InstancesTotal, asts)
			c.Submit("updog.instances_up", ass.InstancesUp, asts)
			c.Submit("updog.instances_failed", ass.InstancesFailed, asts)
			for an, a := range ass.Applications {
				ac := c.NewClient(map[string]string{"application": an})
				ats := a.TimeStamp
				ac.Submit("updog.application.failed", a.Failed, ats)
				ac.Submit("updog.application.degraded", a.Degraded, ats)
				ac.Submit("updog.application.services_total", a.ServicesTotal, ats)
				ac.Submit("updog.application.services_up", a.ServicesUp, ats)
				ac.Submit("updog.application.services_degraded", a.ServicesDegraded, ats)
				ac.Submit("updog.application.services_failed", a.ServicesFailed, ats)
				ac.Submit("updog.application.instances_total", a.InstancesTotal, ats)
				ac.Submit("updog.application.instances_up", a.InstancesUp, ats)
				ac.Submit("updog.application.instances_failed", a.InstancesFailed, ats)
				for sn, s := range a.Services {
					sc := ac.NewClient(map[string]string{"service": sn})
					sts := s.TimeStamp
					sc.Submit("updog.service.failed", s.Failed, sts)
					sc.Submit("updog.service.degraded", s.Degraded, sts)
					sc.Submit("updog.service.instances_total", s.InstancesTotal, sts)
					sc.Submit("updog.service.instances_up", s.InstancesUp, sts)
					sc.Submit("updog.service.instances_failed", s.InstancesFailed, sts)
					for in, i := range s.Instances {
						ic := sc.NewClient(map[string]string{"instance": in})
						its := i.TimeStamp
						ic.Submit("updog.instance.up", i.Up, its)
						ic.Submit("updog.instance.response_time", i.ResponseTime, its)
					}
				}
			}
		}
	}()
	return nil
}
