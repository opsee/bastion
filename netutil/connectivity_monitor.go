package netutil

import "github.com/bjdean/gonetcheck"
import "time"

func CanHasInterweb() (hasInternet bool, err []error) {
	timeoutDuration := time.Duration(10 * time.Second)
	checkUrls := []string{"http://signup.opsee.co", "http://google.com", "http://a1.g.akamaitech.net/"}
	checkAddrs := []string{}
	return gonetcheck.CheckInternetAccess(timeoutDuration, checkUrls, checkAddrs)
}
