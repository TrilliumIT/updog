package dashboard

import (
	updog "github.com/TrilliumIT/updog/types"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
	"strings"
)

func (d *Dashboard) statusHandler(w http.ResponseWriter, r *http.Request) {
	depth := uint64(255)
	var err error
	if r.URL.Query().Get("depth") != "" {
		depth, err = strconv.ParseUint(r.URL.Query().Get("depth"), 10, 8)
		if err != nil {
			log.WithError(err).Error("Error parsing depth value")
			http.Error(w, "Error parsing full", 400)
			return
		}
	}

	parts := strings.SplitN(strings.Trim(r.URL.Path, "/"), "/", 6)
	if len(parts) < 3 {
		parts = []string{"api", "status", "applications"}
	}
	switch parts[2] {
	case "applications":
		returnJson(d.conf.Applications.GetStatus(uint8(depth)), w)
	case "application":
		getAppStatus(parts[3:], uint8(depth), d.conf, w, r)
	default:
		http.NotFound(w, r)
	}
}

func getAppStatus(parts []string, depth uint8, conf *updog.Config, w http.ResponseWriter, r *http.Request) {
	log.WithField("lenparts", len(parts)).WithField("parts", parts).Debug("appstatus")
	app, svc, inst, ok := fromParts(conf, parts)
	switch {
	case !ok:
		http.NotFound(w, r)
	case inst != nil:
		returnJson(inst.GetStatus(depth), w)
	case svc != nil:
		returnJson(svc.GetStatus(depth), w)
	case app != nil:
		returnJson(app.GetStatus(depth), w)
	default:
		http.NotFound(w, r)
	}
}

func fromParts(conf *updog.Config, parts []string) (app *updog.Application, svc *updog.Service, inst *updog.Instance, ok bool) {
	if len(parts) >= 1 {
		app, ok = conf.Applications.Applications[parts[0]]
		if !ok {
			return
		}
	}
	if len(parts) >= 2 {
		svc, ok = app.Services[parts[1]]
		if !ok {
			return
		}
	}
	if len(parts) >= 3 {
		instanceNum := -1
		log.WithField("len(inst)", len(svc.Instances)).WithField("inst", svc.Instances).Debug("Wtf")
		log.WithField("len(parts)", len(parts)).WithField("parts", parts).Debug("Wtf")
		for i, n := range svc.Instances {
			if n.Address() == parts[2] {
				instanceNum = i
				break
			}
		}
		if instanceNum < 0 || instanceNum > len(svc.Instances) {
			return
		}
		ok = true
		inst = svc.Instances[instanceNum]
	}
	return
}
