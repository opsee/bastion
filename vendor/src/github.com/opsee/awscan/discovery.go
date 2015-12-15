package awscan

import (
	"sync"
)

type Discoverer interface {
	Discover() <-chan Event
}

type Event struct {
	Result interface{}
	Err    error
}

type DiscoveryError struct {
	err  error
	Type string
}

type discoverer struct {
	wg        *sync.WaitGroup
	sc        EC2Scanner
	discoChan chan Event
}

const (
	InstanceType         = "Instance"
	DBInstanceType       = "DBInstance"
	SecurityGroupType    = "SecurityGroup"
	DBSecurityGroupType  = "DBSecurityGroup"
	AutoScalingGroupType = "AutoScalingGroup"
	LoadBalancerType     = "LoadBalancerDescription"
	SubnetType           = "Subnet"
	RouteTableType       = "RouteTable"
)

func NewDiscoverer(s EC2Scanner) Discoverer {
	disco := &discoverer{
		sc: s,
		wg: &sync.WaitGroup{},
	}

	return disco
}

func (d *discoverer) doScan(scan func()) {
	d.wg.Add(1)
	go func() {
		scan()
		d.wg.Done()
	}()
}

func (d *discoverer) Discover() <-chan Event {
	d.discoChan = make(chan Event, 128)

	d.doScan(d.scanRouteTables)
	d.doScan(d.scanSubnets)
	d.doScan(d.scanLoadBalancers)
	d.doScan(d.scanRDS)
	d.doScan(d.scanRDSSecurityGroups)
	d.doScan(d.scanSecurityGroups)
	d.doScan(d.scanAutoScalingGroups)

	go func() {
		d.wg.Wait()
		close(d.discoChan)
	}()

	return d.discoChan
}

func (d *discoverer) scanSecurityGroups() {
	if sgs, err := d.sc.ScanSecurityGroups(); err != nil {
		d.discoChan <- Event{nil, &DiscoveryError{err, SecurityGroupType}}
	} else {
		for _, sg := range sgs {
			if sg != nil {
				d.discoChan <- Event{sg, nil}
				if sg.GroupId != nil {
					if reservations, err := d.sc.ScanSecurityGroupInstances(*sg.GroupId); err != nil {
						d.discoChan <- Event{nil, &DiscoveryError{err, InstanceType}}
					} else {
						for _, reservation := range reservations {
							if reservation != nil {
								for _, instance := range reservation.Instances {
									if instance != nil {
										d.discoChan <- Event{instance, nil}
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

func (d *discoverer) scanLoadBalancers() {
	if lbs, err := d.sc.ScanLoadBalancers(); err != nil {
		d.discoChan <- Event{nil, &DiscoveryError{err, LoadBalancerType}}
	} else {
		for _, lb := range lbs {
			if lb != nil {
				d.discoChan <- Event{lb, nil}
			}
		}
	}
}

func (d *discoverer) scanRDS() {
	if rdses, err := d.sc.ScanRDS(); err != nil {
		d.discoChan <- Event{nil, &DiscoveryError{err, DBInstanceType}}
	} else {
		for _, rdsInst := range rdses {
			if rdsInst != nil {
				d.discoChan <- Event{rdsInst, nil}
			}
		}
	}
}

func (d *discoverer) scanRDSSecurityGroups() {
	if rdssgs, err := d.sc.ScanRDSSecurityGroups(); err != nil {
		d.discoChan <- Event{nil, &DiscoveryError{err, DBSecurityGroupType}}
	} else {
		for _, rdssg := range rdssgs {
			if rdssg != nil {
				d.discoChan <- Event{rdssg, nil}
			}
		}
	}
}

func (d *discoverer) scanAutoScalingGroups() {
	if asgs, err := d.sc.ScanAutoScalingGroups(); err != nil {
		d.discoChan <- Event{nil, &DiscoveryError{err, AutoScalingGroupType}}
	} else {
		for _, asg := range asgs {
			if asg != nil {
				d.discoChan <- Event{asg, nil}
			}
		}
	}
}

func (d *discoverer) scanRouteTables() {
	if rts, err := d.sc.ScanRouteTables(); err != nil {
		d.discoChan <- Event{nil, &DiscoveryError{err, RouteTableType}}
	} else {
		for _, rt := range rts {
			if rt != nil {
				d.discoChan <- Event{rt, nil}
			}
		}
	}
}

func (d *discoverer) scanSubnets() {
	if subnets, err := d.sc.ScanSubnets(); err != nil {
		d.discoChan <- Event{nil, &DiscoveryError{err, SubnetType}}
	} else {
		for _, subnet := range subnets {
			if subnet != nil {
				d.discoChan <- Event{subnet, nil}
			}
		}
	}
}

func (e *DiscoveryError) Error() string {
	return e.err.Error()
}
