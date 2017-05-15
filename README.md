# UpDog
What's UpDog?

Nothing, what's up with you?

UpDog is a very simple hierarchical application monitoring service. The design is intended for monitoring clustered (think hadoop) type of applications/services which have a built in level of fault tolerance.

In UpDog's configuration, you define "Applications", which are made up of a bunch of "Services", which in turn are made up of several "Instances". "Services" have check intervals and max failures defined. "Instances" are understood as the same daemon running on multiple nodes, with some level of redundancy. A "Service" becomes degraded if `instance_failures > 0 && instance_faulres <= maxFailures`. See `config_sample.yaml` for a better idea.

We are currently in the very early stages, but the plan is to have a very simple http based dashboard for viewing current application status, and a relay of application status into bosun for alerting rules.
