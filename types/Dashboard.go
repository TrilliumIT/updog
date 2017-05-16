package updog

import (
	log "github.com/sirupsen/logrus"
	"net/http"
)

type Dashboard struct{}

func (d *Dashboard) Start() error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		l := log.WithField("url", r.URL.Path)
		l.Debug("dashboard request")
	})

	log.Info("Starting dashboard listener...")
	return http.ListenAndServe(":8080", nil)
}
