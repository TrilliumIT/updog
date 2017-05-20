package dashboard

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type streamer struct {
	notifier       chan *interface{}
	newClients     chan chan *interface{}
	closingClients chan chan *interface{}
	clients        map[chan *interface{}]struct{}
}

func newStreamer() *streamer {
	s := &streamer{
		notifier:       make(chan *interface{}),
		newClients:     make(chan chan *interface{}),
		closingClients: make(chan chan *interface{}),
		clients:        make(map[chan *interface{}]struct{}),
	}
	go func() {
		for {
			select {
			case c := <-s.newClients:
				s.clients[c] = struct{}{}
			case c := <-s.closingClients:
				delete(s.clients, c)
			case ss := <-s.notifier:
				for c := range s.clients {
					go func(c chan *interface{}, ss *interface{}) {
						c <- ss
					}(c, ss)
				}
			}
		}
	}()
	return s
}

func (broker *streamer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-json-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	messageChan := make(chan *interface{})
	broker.newClients <- messageChan

	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		broker.closingClients <- messageChan
		close(messageChan)
	}()

	for ss := range messageChan {
		if err := json.NewEncoder(w).Encode(ss); err != nil {
			log.WithError(err).WithField("ss", ss).Error("Error encoding json")
			http.Error(w, "Failed to encode json", 500)
			continue
		}
		flusher.Flush()
	}
}
