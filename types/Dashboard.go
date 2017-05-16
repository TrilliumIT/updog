package updog

import (
	log "github.com/sirupsen/logrus"
	"net/http"
)

type Dashboard struct{}

//go:generate go-bindata -prefix "pub/" -pkg updog -o bindata.go pub/...
func (d *Dashboard) Start() error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path
		l := log.WithField("url", url)
		l.Debug("dashboard request")

		a, err := Asset(url)
		if err != nil {
			l.Error("failed to get asset")
		}
		w.Write(a)
	})

	log.Info("Starting dashboard listener...")
	return http.ListenAndServe(":8080", nil)
}
