package utils

import (
	"io"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
)

//DiscardCloseBody takes an io.ReadCloser and reads and discards any data left, then closes it
func DiscardCloseBody(body io.ReadCloser) {
	//discard remaining response body bytes
	b, err := io.Copy(ioutil.Discard, body)
	if err != nil {
		log.WithError(err).Error("Error while closing response body")
	}
	log.WithField("bytes", b).Debug("bytes copied")

	//close response body
	err = body.Close()
	if err != nil {
		log.WithError(err).Error("Error while closing response body")
	}
}
