package heart

import (
	"testing"

	log "github.com/Sirupsen/logrus"
)

func TestHeartMetrics(t *testing.T) {
	_, err := NewHeart("hearttest")
	if err != nil {
		log.Fatal(err.Error())
	}
	metrics, err := Metrics()
	if err != nil || len(metrics) == 0 {
		t.FailNow()
	} else {
		log.Info(metrics)
	}
}