package heart

import (
	"encoding/json"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/opsee/bastion/config"
)

func TestHeartMetrics(t *testing.T) {
	_, err := NewHeart(config.GetConfig(), "hearttest")
	if err != nil {
		log.Fatal(err.Error())
	}
	metrics, err := Metrics()
	if err != nil || len(metrics) == 0 {
		t.FailNow()
	} else {
		log.Info(metrics)
	}

	_, err = json.Marshal(metrics)
	if err != nil {
		t.FailNow()
	}
}
