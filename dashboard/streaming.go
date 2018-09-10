package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	updog "github.com/TrilliumIT/updog/types"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

func (d *Dashboard) streamingWSHandler(w http.ResponseWriter, r *http.Request) {
	d.streamingHandler(w, r, true)
}

func (d *Dashboard) streamingPolHandler(w http.ResponseWriter, r *http.Request) {
	d.streamingHandler(w, r, false)
}

func (d *Dashboard) streamingHandler(w http.ResponseWriter, r *http.Request, ws bool) {
	depth := uint64(255)
	var err error
	if r.URL.Query().Get("depth") != "" { //nolint: dupl
		depth, err = strconv.ParseUint(r.URL.Query().Get("depth"), 10, 8)
		if err != nil {
			log.WithError(err).Error("Error parsing depth value")
			http.Error(w, "Error parsing full", 400)
			return
		}
	}

	full := false
	if r.URL.Query().Get("full") != "" { //nolint: dupl
		full, err = strconv.ParseBool(r.URL.Query().Get("full"))
		if err != nil {
			log.WithError(err).Error("Error parsing full value")
			http.Error(w, "Error parsing full", 400)
			return
		}
	}

	refresh := time.Duration(0)
	if r.URL.Query().Get("refresh") != "" { //nolint: dupl
		refresh, err = time.ParseDuration(r.URL.Query().Get("refresh"))
		if err != nil {
			log.WithError(err).Error("Error parsing refresh value")
			http.Error(w, "Error parsing full", 400)
			return
		}
	}

	onlyChanges := false
	if r.URL.Query().Get("only_changes") != "" { //nolint: dupl
		onlyChanges, err = strconv.ParseBool(r.URL.Query().Get("only_changes"))
		if err != nil {
			log.WithError(err).Error("Error parsing only_changes value")
			http.Error(w, "Error parsing only_changes", 400)
			return
		}
	}

	parts := strings.SplitN(strings.Trim(r.URL.Path, "/"), "/", 6)
	if len(parts) < 3 {
		parts = []string{"api", "streaming", "applications"}
	}

	switch parts[2] {
	case "applications":
		streamJSON(d.conf.Applications, full, uint8(depth), refresh, onlyChanges, ws, w, r)
	case "application":
		streamAppStatus(parts[3:], d.conf, full, uint8(depth), refresh, onlyChanges, ws, w, r)
	default:
		http.NotFound(w, r)
	}
}

func streamAppStatus(parts []string, conf *updog.Config, full bool, depth uint8, maxStale time.Duration, onlyChanges, ws bool, w http.ResponseWriter, r *http.Request) {
	log.WithField("lenparts", len(parts)).WithField("parts", parts).Debug("appstatus")
	app, svc, inst, ok := fromParts(conf, parts)
	switch {
	case !ok:
		http.NotFound(w, r)
	case inst != nil:
		streamJSON(inst, full, depth, maxStale, onlyChanges, ws, w, r)
	case svc != nil:
		streamJSON(svc, full, depth, maxStale, onlyChanges, ws, w, r)
	case app != nil:
		streamJSON(app, full, depth, maxStale, onlyChanges, ws, w, r)
	default:
		http.NotFound(w, r)
	}
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func streamJSON(subr updog.Subscriber, full bool, depth uint8, maxStale time.Duration, onlyChanges, ws bool, w http.ResponseWriter, r *http.Request) {
	var process func(interface{}) error

	if ws {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			log.WithError(err).Error("Error upgrading websocket connection")
			return
		}
		process = func(d interface{}) error {
			l := log.WithField("data", d)
			var wsw io.WriteCloser
			wsw, err = conn.NextWriter(websocket.TextMessage)
			if err != nil {
				// Connection closed
				//log.WithError(err).Error("Error getting writer from ws conn")
				return nil
			}
			if err = json.NewEncoder(wsw).Encode(d); err != nil {
				l.WithError(err).Error("Error encoding json")
				http.Error(w, "Failed to encode json", 500)
				return err
			}
			return nil
		}
	} else {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		process = func(d interface{}) error {
			l := log.WithField("data", d)
			b, err := w.Write([]byte("data: "))
			if err != nil {
				return err
			}
			l.WithField("bytes", b).Debug("bytes written")
			if err = json.NewEncoder(w).Encode(d); err != nil {
				l.WithError(err).Error("Error encoding json")
				http.Error(w, "Failed to encode json", 500)
				return err
			}
			b, err = w.Write([]byte("\n\n"))
			if err != nil {
				return err
			}
			l.WithField("bytes", b).Debug("bytes written")
			flusher.Flush()
			return nil
		}
	}

	sub := subr.Sub(full, depth, maxStale, onlyChanges)

	go func() {
		<-r.Context().Done()
		sub.Close()
	}()

	for {
		d := sub.Next()
		select {
		case <-r.Context().Done():
			return
		default:
		}
		l := log.WithField("data", d)

		if err := process(d); err != nil {
			l.WithError(err).Error("Error from process function")
			return
		}
	}
}
