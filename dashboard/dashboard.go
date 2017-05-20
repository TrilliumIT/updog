package dashboard

import (
	"encoding/json"
	updog "github.com/TrilliumIT/updog/types"
	"github.com/nytimes/gziphandler"
	log "github.com/sirupsen/logrus"
	"mime"
	"net/http"
	"path"
	"path/filepath"
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
	http.Handle("/", gziphandler.GzipHandler(http.HandlerFunc(d.rootHandler)))
	http.Handle("/api/config", jsonHandler(func() interface{} { return d.conf }))

	http.Handle("/api/applications", jsonHandler(func() interface{} { return d.conf.Applications.GetStatus() }))
	http.Handle("/api/application/", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			jsonHandler(func() interface{} { return d.conf.Applications.Applications[path.Base(r.URL.Path)].GetStatus() })
		}))

	http.Handle("/api/app/emby", jsonHandler(func() interface{} {
		return d.conf.Applications.Applications["emby"].GetStatus()
	}))
	http.Handle("/api/svc/emby", jsonHandler(func() interface{} {
		return d.conf.Applications.Applications["emby"].Services["emby"].GetStatus()
	}))
	http.Handle("/api/inst/emby", jsonHandler(func() interface{} {
		return d.conf.Applications.Applications["emby"].Services["emby"].Instances[0].GetStatus()
	}))
	log.Info("Starting dashboard listener...")
	return http.ListenAndServe(":8080", nil)
}

func jsonHandler(get func() interface{}) http.Handler {
	return gziphandler.GzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		d := get()
		if err := json.NewEncoder(w).Encode(get()); err != nil {
			log.WithError(err).WithField("d", d).Error("Error encoding json")
			http.Error(w, "Failed to encode json", 500)
			return
		}
	}))
}

func (d *Dashboard) rootHandler(w http.ResponseWriter, r *http.Request) {
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
