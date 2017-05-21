package dashboard

import (
	"encoding/json"
	updog "github.com/TrilliumIT/updog/types"
	"github.com/nytimes/gziphandler"
	log "github.com/sirupsen/logrus"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

//go:generate go-bindata -prefix "pub/" -pkg dashboard -o bindata.go pub/...
type Dashboard struct {
	conf *updog.Config
}

func NewDashboard(conf *updog.Config) *Dashboard {
	log.WithField("conf", conf).Info("Creating dashboard")
	return &Dashboard{conf: conf}
}

func (d *Dashboard) Start() error {
	log.Info("Starting dashboard listener...")
	return http.ListenAndServe(":8080", d)
}

func (d *Dashboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := strings.Trim(r.URL.Path, "/")
	switch {
	case p == "":
		gziphandler.GzipHandler(http.HandlerFunc(d.rootHandler)).ServeHTTP(w, r)
	case strings.HasPrefix(p, "api/"):
		gziphandler.GzipHandler(http.HandlerFunc(d.apiHandler)).ServeHTTP(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (d *Dashboard) apiHandler(w http.ResponseWriter, r *http.Request) {
	p := strings.Trim(r.URL.Path, "/")
	switch {
	case p == "api/config":
		returnJson(d.conf, w)
	case p == "api/applications":
		http.Redirect(w, r, "/api/status/applications", 301)
	case strings.HasPrefix(p, "api/status"):
		d.statusHandler(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (d *Dashboard) statusHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.Trim(r.URL.Path, "/"), "/", 6)
	if len(parts) < 3 {
		parts = []string{"api", "status", "applications"}
	}
	switch parts[2] {
	case "applications":
		returnJson(d.conf.Applications.GetStatus(), w)
	case "application":
		getAppStatus(parts[3:], d.conf, w, r)
	default:
		http.NotFound(w, r)
	}
}

func getAppStatus(parts []string, conf *updog.Config, w http.ResponseWriter, r *http.Request) {
	log.WithField("lenparts", len(parts)).WithField("parts", parts).Debug("appstatus")
	switch len(parts) {
	case 1:
		app, ok := conf.Applications.Applications[parts[0]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		returnJson(app.GetStatus(), w)
	case 2:
		svc, ok := conf.Applications.Applications[parts[0]].Services[parts[1]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		returnJson(svc.GetStatus(), w)
	case 3:
		instanceNum := -1
		for i, n := range conf.Applications.Applications[parts[0]].Services[parts[1]].Instances {
			if n.Address() == parts[2] {
				instanceNum = i
				break
			}
		}
		if instanceNum < 0 || instanceNum > len(conf.Applications.Applications[parts[0]].Services[parts[1]].Instances) {
			http.NotFound(w, r)
			return
		}
		inst := conf.Applications.Applications[parts[0]].Services[parts[1]].Instances[instanceNum]
		returnJson(inst.GetStatus(), w)
	default:
		http.NotFound(w, r)
	}
}

func returnJson(d interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(d); err != nil {
		log.WithError(err).WithField("d", d).Error("Error encoding json")
		http.Error(w, "Failed to encode json", 500)
	}
}

func jsonHandler(get func() interface{}) http.Handler {
	return gziphandler.GzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		returnJson(get(), w)
	}))
}

func (d *Dashboard) rootHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("root")
	url := r.URL.Path[1:]
	l := log.WithField("url", url)
	l.Debug("dashboard request")

	if url == "" {
		url = "index.html"
	}

	a, err := Asset(url)
	if err != nil {
		http.NotFound(w, r)
		l.Error("failed to load asset")
		return
	}
	f, err := AssetInfo(url)
	if err != nil {
		http.NotFound(w, r)
		l.Error("failed to load asset info")
		return
	}

	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(f.Name())))
	w.Write(a)
}
