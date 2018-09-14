package dashboard

import (
	"encoding/json"
	"mime"
	"net/http"
	_ "net/http/pprof" //http profiler
	"path/filepath"
	"strings"

	"github.com/NYTimes/gziphandler"
	updog "github.com/TrilliumIT/updog/types"
	log "github.com/sirupsen/logrus"
)

//go:generate go-bindata -prefix "pub/" -pkg dashboard -o bindata.go pub/...

//Dashboard is the main listening web server type
type Dashboard struct {
	conf *updog.Config
	ah   http.Handler
	rh   http.Handler
}

//NewDashboard creates a new dashboard
func NewDashboard(conf *updog.Config) *Dashboard {
	log.WithField("conf", conf).Info("Creating dashboard")
	return &Dashboard{conf: conf}
}

//Start starts the dashboard
func (d *Dashboard) Start() error {
	log.Info("Starting dashboard listener...")
	go func() {
		err := http.ListenAndServe(":8081", nil)
		if err != nil {
			log.WithField("dashboard", d).Error("Error Listening on :8081")
		}
	}()

	gzHandler, err := gziphandler.GzipHandlerWithOpts(gziphandler.MinSize(1))
	if err != nil {
		return err
	}
	d.ah = gzHandler(http.HandlerFunc(d.apiHandler))
	d.rh = gzHandler(http.HandlerFunc(d.rootHandler))

	return http.ListenAndServe(":8080", d)
}

//ServeHTTP serves the request
func (d *Dashboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := strings.Trim(r.URL.Path, "/")
	switch {
	case strings.HasPrefix(p, "api/"):
		//gziphandler.GzipHandler(http.HandlerFunc(d.apiHandler)).ServeHTTP(w, r)
		//http.HandlerFunc(d.apiHandler).ServeHTTP(w, r)
		d.ah.ServeHTTP(w, r)
	default:
		//gziphandler.GzipHandler(http.HandlerFunc(d.rootHandler)).ServeHTTP(w, r)
		//http.HandlerFunc(d.rootHandler).ServeHTTP(w, r)
		d.rh.ServeHTTP(w, r)
	}
}

func (d *Dashboard) apiHandler(w http.ResponseWriter, r *http.Request) {
	p := strings.Trim(r.URL.Path, "/")
	switch {
	case p == "api/config":
		returnJSON(d.conf, w)
	case p == "api/applications":
		http.Redirect(w, r, "/api/status/applications", 301)
	case strings.HasPrefix(p, "api/status"):
		d.statusHandler(w, r)
	case strings.HasPrefix(p, "api/streaming"):
		d.streamingPolHandler(w, r)
	case strings.HasPrefix(p, "api/ws"):
		d.streamingWSHandler(w, r)
	default:
		http.NotFound(w, r)
	}
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
	var b int
	b, err = w.Write(a)
	if err != nil {
		l.Error("Error writing content")
	}
	l.WithField("bytes", b).Debug("Wrote bytes")
}

func returnJSON(d interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(d); err != nil {
		log.WithError(err).WithField("d", d).Error("Error encoding json")
		http.Error(w, "Failed to encode json", 500)
	}
}
