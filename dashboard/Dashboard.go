package dashboard

import (
	log "github.com/sirupsen/logrus"
	"net/http"
)

//go:generate go-bindata -prefix "pub/" -pkg dashboard -o bindata.go pub/...
type Dashboard struct{}

func (d *Dashboard) Start() error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path[1:]
		l := log.WithField("url", url)
		l.Debug("dashboard request")

		a, err := Asset(url)
		if err != nil {
			http.NotFound(w, r)
			l.Error("failed to load asset")
			return
		}

		w.Write(a)
	})

	http.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path[1:]
		l := log.WithField("url", url)
		l.Debug(url)
	})

	log.Info("Starting dashboard listener...")
	return http.ListenAndServe(":8080", nil)
}
