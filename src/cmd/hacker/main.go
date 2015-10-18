package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/opsee/awscan"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
)

var (
	moduleName   = "hacker"
	posixSignals = make(chan os.Signal, 1)
	bastionSgId  = aws.String("none")
	sigs         = make(chan os.Signal, 2)
	httpClient   = &http.Client{}
	ec2Client    = &ec2.EC2{}
	heartbeat    = &heart.Heart{}
	sc           awscan.EC2Scanner
)

// signal is a POSIX portable int to add context for state transitions
type Signal struct {
	ID  string
	VAL int
}

// discrete state consists of ID and a function behavior
type State struct {
	ID       string
	Behavior func(fsm *fsm)
}

// abstract finite state machine's state and behavior
type fsm struct {
	Signals map[string]*Signal
	States  map[string]*State
	STATE   *State
}

// Operations on an abstract finite state machine
type FSM interface {
	GetSignals() map[string]*Signal
	GetStates() map[string]*State
	GetState() *State
	Transition(state *State, signal *Signal) *State // a special state.
	Execute(state *State)
}

// execute a state and set fsm's state to that state
func (fsm *fsm) Execute(state *State) {
	log.WithFields(log.Fields{"state": fsm.GetState().ID}).Info("execution complete.")
	fsm.SetState(state)
	log.WithFields(log.Fields{"state": fsm.GetState().ID}).Info("executing.")
	state.Behavior(fsm)
}

// set the fsm's state
func (fsm *fsm) SetState(state *State) {
	fsm.STATE = state
}

// returns pointer to current state
func (fsm *fsm) GetState() *State {
	return fsm.STATE
}

// returns pointer to current state
func (fsm *fsm) GetStates() map[string]*State {
	return fsm.States
}

// returns pointer to current state
func (fsm *fsm) GetSignal(ID string) *Signal {
	return fsm.Signals[ID]
}

// returns pointer to current state
func (fsm *fsm) GetSignals() map[string]*Signal {
	return fsm.Signals
}

// Initial State, Check Flags, Setup Vars
func Startup(fsm *fsm) {
	// check to see if we're supposed to be adding ingress
	if os.Getenv("ENABLE_BASTION_INGRESS") == "false" {
		log.WithFields(log.Fields{"service": "monitor"}).Info("ENABLE_BASTION_INGRESS set false.  exiting.")
		os.Exit(0)
	}

	// get config, initialize AWSCAN
	cfg := config.GetConfig()
	sc = awscan.NewScanner(&awscan.Config{AccessKeyId: cfg.AccessKeyId, SecretKey: cfg.SecretKey, Region: cfg.MetaData.Region})

	var creds = credentials.NewChainCredentials(
		[]credentials.Provider{
			&credentials.StaticProvider{Value: credentials.Value{
				AccessKeyID:     cfg.AccessKeyId,
				SecretAccessKey: cfg.SecretKey,
				SessionToken:    "",
			}},
			&credentials.EnvProvider{},
			&ec2rolecreds.EC2RoleProvider{ExpiryWindow: 5 * time.Minute},
		})

	awsConfig := &aws.Config{Credentials: creds, Region: aws.String(cfg.MetaData.Region)}
	ec2Client = ec2.New(awsConfig)

	resp, err := httpClient.Get("http://169.254.169.254/latest/meta-data/security-groups/")
	if err != nil {
		log.WithFields(log.Fields{"state": fsm.GetState().ID, "err": err.Error()}).Fatal("Unable to get security group from meta data service")
		os.Exit(1)
	}

	defer resp.Body.Close()
	secGroupName, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err.Error())
		log.WithFields(log.Fields{"state": fsm.GetState().ID, "err": err.Error()}).Fatal("Error reading response body while getting security group name")
		os.Exit(1)
	}

	output, err := ec2Client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name: aws.String("group-name"),
				Values: []*string{
					aws.String(string(secGroupName)),
				},
			},
		},
	})
	if err != nil {
		log.WithFields(log.Fields{"state": fsm.GetState().ID, "err": err.Error()}).Fatal("ec2 client error getting security groups input")
		os.Exit(1)
	}

	if len(output.SecurityGroups) != 1 {
		log.WithFields(log.Fields{"state": fsm.GetState().ID, "err": err.Error()}).Fatal("bad number of groups found")
		os.Exit(1)
	}

	bastionSgId = output.SecurityGroups[0].GroupId
}

// concrete transition function
func (fsm *fsm) Transition(state *State, signal *Signal) *State {
	/*
	* STATE Startup
	* LOOPA := STATE Hack->STATE Wait->STATE Hack->STATE Wait...
	* LOOPA until SIGNAL == SIGTERM | SIGKILL | SIGINT | SIQUIT
	* STATE UnHack
	* END
	 */
	switch state.ID {
	case "STARTUP":
		switch signal.ID {
		case "SIGTERM", "SIGKILL", "SIGINT", "SIGQUIT":
			os.Exit(1)
		case "_SIGNOSIGNAL":
			return fsm.GetStates()["HACK"]
		}
	case "HACK":
		switch signal.ID {
		case "SIGTERM", "SIGKILL", "SIGINT", "SIGQUIT":
			return fsm.GetStates()["UNHACK"]
		case "_SIGNOSIGNAL":
			return fsm.GetStates()["WAIT"]
		}
	case "WAIT":
		switch signal.ID {
		case "SIGTERM", "SIGKILL", "SIGINT", "SIGQUIT":
			// always attempt to unhack before dying
			return fsm.GetStates()["UNHACK"]
		case "_SIGNOSIGNAL":
			return fsm.GetStates()["HACK"]
		}
	case "UNHACK":
		switch signal.ID {
		case "SIGTERM", "SIGKILL", "SIGINT", "SIGQUIT":
			os.Exit(1)
		case "_SIGNOSIGNAL":
			os.Exit(0)
		}
	}
	return nil
}

// remove self from security groups
func UnHack(fsm *fsm) {
	ippermission := []*ec2.IpPermission{
		&ec2.IpPermission{
			IpProtocol: aws.String("-1"),
			FromPort:   aws.Int64(0),
			ToPort:     aws.Int64(65535),
			UserIdGroupPairs: []*ec2.UserIdGroupPair{
				&ec2.UserIdGroupPair{
					GroupId: bastionSgId,
				},
			},
		}}

	sgs, err := sc.ScanSecurityGroups()

	if err != nil {
		log.WithFields(log.Fields{"state": fsm.GetState().ID, "err": err.Error()}).Error("security group discovery error")
	}

	for _, sg := range sgs {
		_, err := ec2Client.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
			GroupId:       sg.GroupId,
			IpPermissions: ippermission,
		})
		if err != nil {
			log.WithFields(log.Fields{"state": fsm.GetState().ID, "err": err.Error()}).Error("bastion failed to pwn: ", sg.GroupId)
		} else {
			log.WithFields(log.Fields{"state": fsm.GetState().ID}).Info("bastion pwned: ", sg.GroupId)
		}

		// XXX
		select {
		case s := <-posixSignals:
			switch s {
			case syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGQUIT:
				fsm.Execute(fsm.Transition(fsm.GetState(), fsm.GetSignals()["SIGTERM"]))
			}
		case <-time.After(1 * time.Second):
			return // break to next state
		}
	}
}

func Wait(fsm *fsm) {
	for {
		select {

		// XXX
		case s := <-posixSignals:
			switch s {
			case syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGQUIT:
				fsm.Execute(fsm.Transition(fsm.GetState(), fsm.GetSignals()["SIGTERM"]))
			default:
				fsm.Execute(fsm.Transition(fsm.GetState(), fsm.GetSignals()["_SIGNOSIGNAL"]))
			}
		case <-time.After(25 * time.Minute):
			return // break to next state
		}
	}
}

func Hack(fsm *fsm) {
	ippermission := []*ec2.IpPermission{
		&ec2.IpPermission{
			IpProtocol: aws.String("-1"),
			FromPort:   aws.Int64(0),
			ToPort:     aws.Int64(65535),
			UserIdGroupPairs: []*ec2.UserIdGroupPair{
				&ec2.UserIdGroupPair{
					GroupId: bastionSgId,
				},
			},
		}}

	sgs, err := sc.ScanSecurityGroups()

	if err != nil {
		log.WithFields(log.Fields{"state": fsm.GetState().ID, "err": err.Error()}).Error("security group discovery error")
	}

	for _, sg := range sgs {
		ingressRuleFound := false
		for _, perm := range sg.IpPermissions {
			for _, ipr := range perm.IpRanges {
				if ipr.CidrIp == bastionSgId {
					ingressRuleFound = true
				}
			}
		}

		// if a rule doesn't yet exist for our bastion, create one
		if !ingressRuleFound {
			_, err := ec2Client.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
				GroupId:       sg.GroupId,
				IpPermissions: ippermission,
			})
			if err != nil {
				log.WithFields(log.Fields{"state": fsm.GetState().ID, "err": err.Error()}).Error("bastion failed to pwn: ", sg.GroupId)
			} else {
				log.WithFields(log.Fields{"state": fsm.GetState().ID}).Info("bastion pwned: ", sg.GroupId)
			}
		}

		// XXX
		select {
		case s := <-posixSignals:
			switch s {
			case syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGQUIT:
				fsm.Execute(fsm.Transition(fsm.GetState(), fsm.GetSignals()["SIGTERM"]))
			}
		case <-time.After(1 * time.Second):
			return // break to next state
		}
	}
}

func main() {
	signal.Notify(posixSignals,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	// POSIX Signals and one special _SIGNOSIGNAL
	// _SIGNOSIGNAL also represents unknown signals
	signals := map[string]*Signal{
		"SIGHUP":       &Signal{ID: "SIGHUP", VAL: 1},
		"SIGINT":       &Signal{ID: "SIGINT", VAL: 2},
		"SIGQUIT":      &Signal{ID: "SIGQUIT", VAL: 3},
		"SIGTERM":      &Signal{ID: "SIGTERM", VAL: 15},
		"SIGABRT":      &Signal{ID: "SIGABRT", VAL: 6},
		"SIGKILL":      &Signal{ID: "SIGKILL", VAL: 9},
		"_SIGNOSIGNAL": &Signal{ID: "_SIGNOSIGNAL", VAL: 0},
	}

	// dahacker's states
	states := map[string]*State{
		"HACK":    &State{ID: "HACK", Behavior: Hack},
		"UNHACK":  &State{ID: "UNHACK", Behavior: UnHack},
		"WAIT":    &State{ID: "WAIT", Behavior: Wait},
		"STARTUP": &State{ID: "STARTUP", Behavior: Startup},
	}

	// initialize dahacker fsm
	dahacker := &fsm{Signals: signals, States: states, STATE: states["STARTUP"]}
	var dafsm FSM
	dafsm = dahacker

	// initialize a heartbeat
	heartbeat, err := heart.NewHeart(moduleName)
	if err != nil {
		log.WithFields(log.Fields{"service": "hacker", "err": err.Error()}).Fatal("Error getting heartbeat")
	} else {
		<-heartbeat.Beat()
	}

	// run the state machine
	for {
		dafsm.Execute(dafsm.Transition(dafsm.GetState(), dafsm.GetSignals()["_SIGNOSIGNAL"]))
	}
}
