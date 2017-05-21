package dashboard

import (
	"encoding/json"
	updog "github.com/TrilliumIT/updog/types"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

func (d *Dashboard) streamingHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.Trim(r.URL.Path, "/"), "/", 6)
	if len(parts) < 3 {
		parts = []string{"api", "streaming", "applications"}
	}
	switch parts[2] {
	case "applications":
		streamJson(d.conf.Applications, w)
	case "application":
		streamAppStatus(parts[3:], d.conf, w, r)
	default:
		http.NotFound(w, r)
	}
}

func streamAppStatus(parts []string, conf *updog.Config, w http.ResponseWriter, r *http.Request) {
	log.WithField("lenparts", len(parts)).WithField("parts", parts).Debug("appstatus")
	app, svc, inst, ok := fromParts(conf, parts)
	switch {
	case !ok:
		http.NotFound(w, r)
	case inst != nil:
		streamJson(inst, w)
	case svc != nil:
		streamJson(svc, w)
	case app != nil:
		streamJson(app, w)
	default:
		http.NotFound(w, r)
	}
}

func streamJson(subr updog.Subscriber, w http.ResponseWriter) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-json-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sub := subr.Sub()

	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		sub.Close()
	}()

	for {
		d := sub.Next()
		select {
		case <-notify:
			return
		default:
		}
		l := log.WithField("data", d)
		l.Debug("Sending streaming update")

		if err := json.NewEncoder(w).Encode(d); err != nil {
			l.WithError(err).Error("Error encoding json")
			http.Error(w, "Failed to encode json", 500)
			return
		}
		flusher.Flush()
	}
}
