package dashboard

import (
	"encoding/json"
	updog "github.com/TrilliumIT/updog/types"
	log "github.com/sirupsen/logrus"
	"net/http"
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
	http.HandleFunc("/", d.rootHandler)
	http.HandleFunc("/api/", d.apiHandler)

	log.Info("Starting dashboard listener...")
	return http.ListenAndServe(":8080", nil)
}

func (d *Dashboard) apiHandler(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Path[5:]
	l := log.WithField("url", url)
	l.Debug("api request")

	if url == "" {
		//TODO: something smart for the root
		return
	}

	parts := strings.Split(url, "/")

	if len(parts) > 1 {
		switch parts[2] {
		case "applications":
			b, err := json.Marshal(d.applications.GetApplicationStatus())
			if err != nil {
				http.Error(w, "Failed to marshal json", 500)
				return
			}
			w.Write(b)
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

	w.Write(a)
}
