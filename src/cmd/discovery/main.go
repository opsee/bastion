package main

import (
	"sync"
	"time"

	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/messaging"
	"github.com/opsee/bastion/scanner"
)

const (
	moduleName = "discovery"
)

var (
	logger   = logging.GetLogger(moduleName)
	producer messaging.Producer
	wg       *sync.WaitGroup
	sc       scanner.EC2Scanner
)

func scanSecurityGroups() {
	wg.Add(1)
	if sgs, err := sc.ScanSecurityGroups(); err != nil {
		logger.Error(err.Error())
	} else {
		for _, sg := range sgs {
			if sg != nil {
				if err := producer.Publish(sg); err != nil {
					logger.Error(err.Error())
				}
				if sg.GroupID != nil {
					logger.Info("Scanning SG: %v", *sg.GroupID)
					if reservations, err := sc.ScanSecurityGroupInstances(*sg.GroupID); err != nil {
						logger.Error(err.Error())
					} else {
						for _, reservation := range reservations {
							if reservation != nil {
								for _, instance := range reservation.Instances {
									if instance != nil {
										if err := producer.Publish(instance); err != nil {
											logger.Error(err.Error())
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	wg.Done()
}

func scanLoadBalancers() {
	wg.Add(1)
	if lbs, err := sc.ScanLoadBalancers(); err != nil {
		logger.Error(err.Error())
	} else {
		for _, lb := range lbs {
			if lb != nil {
				if err := producer.Publish(lb); err != nil {
					logger.Error(err.Error())
				}
			}
		}
	}
	wg.Done()
}

func scanRDS() {
	wg.Add(1)
	if rdses, err := sc.ScanRDS(); err != nil {
		logger.Error(err.Error())
	} else {
		for _, rdsInst := range rdses {
			if rdsInst != nil {
				if err := producer.Publish(rdsInst); err != nil {
					logger.Error(err.Error())
				}
			}
		}
	}
	wg.Done()
}

func scanRDSSecurityGroups() {
	wg.Add(1)
	if rdssgs, err := sc.ScanRDSSecurityGroups(); err != nil {
		logger.Error(err.Error())
	} else {
		for _, rdssg := range rdssgs {
			if rdssg != nil {
				if err := producer.Publish(rdssg); err != nil {
					logger.Error(err.Error())
				}
			}
		}
	}
	wg.Done()
}

func main() {
	var err error

	cfg := config.GetConfig()
	sc = scanner.NewScanner(cfg)
	wg = &sync.WaitGroup{}

	producer, err = messaging.NewProducer("discovery")
	if err != nil {
		panic(err)
	}

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		panic(err)
	}
	go heart.Beat()

	for {
		go scanLoadBalancers()
		go scanRDS()
		go scanRDSSecurityGroups()
		go scanSecurityGroups()
		// Wait for the whole scan to finish.
		wg.Wait()
		// Sleep 2 minutes between scans.
		time.Sleep(120 * time.Second)
	}
}
