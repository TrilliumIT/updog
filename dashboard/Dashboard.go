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
	applications *updog.Applications
}

func NewDashboard(apps *updog.Applications) *Dashboard {
	return &Dashboard{applications: apps}
}

func (d *Dashboard) Start() error {
	http.Handle("/", gziphandler.GzipHandler(http.HandlerFunc(d.rootHandler)))
	http.Handle("/api/", gziphandler.GzipHandler(http.HandlerFunc(d.apiHandler)))

	log.Info("Starting dashboard listener...")
	return http.ListenAndServe(":8080", nil)
}

func (d *Dashboard) apiHandler(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Path[5:]
	l := log.WithField("url", url)
	l.Debug("api request")

	if url == "" {
		http.Error(w, "Invalid Request.", 400)
		return
	}

	parts := strings.Split(url, "/")

	if len(parts) > 0 {
		switch parts[0] {
		case "applications":
			w.Header().Set("Content-Type", "application/json")
			a, err := json.Marshal(d.applications.GetApplicationStatus())
			if err != nil {
				http.Error(w, "Failed to encode json", 500)
				return
			}
			w.Write(a)
		default:
			http.NotFound(w, r)
			return
		}
	}
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
