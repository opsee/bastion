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
	moduleName     = "hacker"
	signalsChannel = make(chan os.Signal, 1)
	bastionSgId    = aws.String("none")
	httpClient     = &http.Client{}
	ec2Client      = &ec2.EC2{}
	heartbeat      = &heart.Heart{}
	sc             awscan.EC2Scanner
)

type SecurityGroupID string

// posible exit codes a hacker's states can have
type StateExitCode int

const (
	StateExitSuccess StateExitCode = 0
	StateExitError   StateExitCode = 1
)

// When a handler handles a signal, it returns one of these codes to the state
// the state reacts accordingly
type StateHandlerCode int

const (
	HANDLE_CONTINUE StateHandlerCode = 0
	HANDLE_EXIT     StateHandlerCode = 1
	HANDLE_RESTART  StateHandlerCode = 2
)

// possible states that a hacker can be in
type StateID string

const (
	STATE_HACK         StateID = "HACK"
	STATE_UNHACK       StateID = "UNHACK"
	STATE_WAIT         StateID = "WAIT"
	STATE_STARTUP      StateID = "STARTUP"
	STATE_EXIT_ERROR   StateID = "STATE_EXIT_ERROR"
	STATE_EXIT_SUCCESS StateID = "STATE_EXIT_SUCCESS"
)

// discrete state consists of ID and a function behavior
type State struct {
	ID       StateID
	Behavior func(fsm *fsm) *StateExitInfo
}

// desired next state and exit code
// all states return this
type StateExitInfo struct {
	NextState StateID
	ExitCode  StateExitCode
}

// abstract finite state machine's state and behavior
// - a collection of states (which contain behaviors)
// - a current state
// TODO - an input channel for events from OS and other FSMs
// TODO - an output channel for publishing events about self to other FSMs
type fsm struct {
	ID     string
	States map[StateID]*State
	STATE  *State
}

// Operations on an abstract finite state machine
type FSM interface {
	GetStates() map[StateID]*State // return map of states
	GetCurrentState() *State       // return current state
	GetState(StateID) *State       // return state by StateID
	SetState(*State)               // set current state
	ExecuteCurrentState() *StateExitInfo

	// an FSM can transition on a signal, or when a state exits
	HandleSignal(state *State, signal os.Signal) StateHandlerCode
}

// set the fsm's state
func (fsm *fsm) SetState(state *State) {
	fsm.STATE = state
}

// returns pointer to current state
func (fsm *fsm) GetCurrentState() *State {
	return fsm.STATE
}

// returns pointer to current state
func (fsm *fsm) GetStates() map[StateID]*State {
	return fsm.States
}

// returns a state by it's const StateID
// TODO handle state not found
func (fsm *fsm) GetState(stateID StateID) *State {
	if state, ok := fsm.States[stateID]; ok {
		return state
	} else {
		log.WithFields(log.Fields{"fsm": fsm.ID, "State": stateID}).Fatal("Couldn't find state!")
		return nil
	}
}

func (fsm *fsm) ExecuteCurrentState() *StateExitInfo {
	log.WithFields(log.Fields{"state": fsm.GetCurrentState().ID}).Info("Executing")
	return fsm.GetCurrentState().Behavior(fsm)
}

// Initial State, Check Flags, Setup Vars
func Startup(fsm *fsm) *StateExitInfo {
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
		log.WithFields(log.Fields{"state": fsm.GetCurrentState().ID, "err": err.Error()}).Fatal("Unable to get security group from meta data service")
		return &StateExitInfo{NextState: STATE_EXIT_ERROR, ExitCode: StateExitError}
	}

	defer resp.Body.Close()
	secGroupName, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err.Error())
		log.WithFields(log.Fields{"state": fsm.GetCurrentState().ID, "err": err.Error()}).Fatal("Error reading response body while getting security group name")
		return &StateExitInfo{NextState: STATE_EXIT_ERROR, ExitCode: StateExitError}
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
		log.WithFields(log.Fields{"state": fsm.GetCurrentState().ID, "err": err.Error()}).Fatal("ec2 client error getting security groups input")
		return &StateExitInfo{NextState: STATE_EXIT_ERROR, ExitCode: StateExitError}
	}

	if len(output.SecurityGroups) != 1 {
		log.WithFields(log.Fields{"state": fsm.GetCurrentState().ID, "err": err.Error()}).Fatal("bad number of groups found")
		return &StateExitInfo{NextState: STATE_EXIT_ERROR, ExitCode: StateExitError}
	}

	bastionSgId = output.SecurityGroups[0].GroupId

	// state's behavior executed successfully
	return &StateExitInfo{NextState: STATE_HACK, ExitCode: StateExitSuccess}
}

// remove self from security groups
// don't handle signals here, we always want to finish unhacking if we can
func UnHack(fsm *fsm) *StateExitInfo {
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
		log.WithFields(log.Fields{"state": fsm.GetCurrentState().ID, "err": err.Error()}).Error("security group discovery error")
	}

	// remove self from indexed security groups
	for _, sg := range sgs {
		_, err := ec2Client.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
			GroupId:       sg.GroupId,
			IpPermissions: ippermission,
		})
		if err != nil {
			log.WithFields(log.Fields{"state": fsm.GetCurrentState().ID, "err": err.Error()}).Error("bastion failed to pwn: ", sg.GroupId)
		} else {
			log.WithFields(log.Fields{"state": fsm.GetCurrentState().ID}).Info("bastion pwned: ", sg.GroupId)
		}
	}

	return &StateExitInfo{NextState: STATE_EXIT_SUCCESS, ExitCode: StateExitSuccess}
}

// wait state, wants to return to hacker
func Wait(fsm *fsm) *StateExitInfo {
	for {
		select {
		case s := <-signalsChannel:
			switch fsm.HandleSignal(fsm.GetCurrentState(), s) {
			case HANDLE_CONTINUE:
				continue // handler tells us to continue
			case HANDLE_EXIT:
				return &StateExitInfo{NextState: STATE_UNHACK, ExitCode: StateExitSuccess} // handler tells us to exit
			case HANDLE_RESTART:
				return &StateExitInfo{NextState: STATE_WAIT, ExitCode: StateExitSuccess}
			}
		case <-time.After(1 * time.Minute):
			return &StateExitInfo{NextState: STATE_HACK, ExitCode: StateExitSuccess} // we're done, go back to hacking
		}
	}
}

// hack state, add ingress rules
func Hack(fsm *fsm) *StateExitInfo {
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
		log.WithFields(log.Fields{"state": fsm.GetCurrentState().ID, "err": err.Error()}).Error("security group discovery error")
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
				log.WithFields(log.Fields{"state": fsm.GetCurrentState().ID, "err": err.Error()}).Error("bastion failed to pwn: ", sg.GroupId)
			} else {
				log.WithFields(log.Fields{"state": fsm.GetCurrentState().ID}).Info("bastion pwned: ", sg.GroupId)
			}
		}

		// see if we've gotten any signals
		select {
		case s := <-signalsChannel:
			switch fsm.HandleSignal(fsm.GetCurrentState(), s) {
			case HANDLE_CONTINUE:
				continue // handler tells us to continue
			case HANDLE_EXIT:
				return &StateExitInfo{NextState: STATE_UNHACK, ExitCode: StateExitSuccess} // handler tells us to exit
			}
		case <-time.After(1 * time.Second):
			continue // continue to next SG
		}
	}

	// finally, go to STATE_WAIT
	return &StateExitInfo{NextState: STATE_WAIT, ExitCode: StateExitSuccess}
}

func main() {
	if os.Getenv("ENABLE_BASTION_INGRESS") == "false" {
		log.WithFields(log.Fields{"service": "hacker"}).Info("ENABLE_BASTION_INGRESS set false.  exiting.")
		os.Exit(0)
	}

	// handle the following signals
	signal.Notify(signalsChannel,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	// dahacker's states
	states := map[StateID]*State{
		STATE_HACK:    &State{ID: STATE_HACK, Behavior: Hack},
		STATE_UNHACK:  &State{ID: STATE_UNHACK, Behavior: UnHack},
		STATE_WAIT:    &State{ID: STATE_WAIT, Behavior: Wait},
		STATE_STARTUP: &State{ID: STATE_STARTUP, Behavior: Startup},
	}

	// initialize dahacker fsm
	dahacker := &fsm{ID: "hackerunhacker", States: states, STATE: states[STATE_STARTUP]}
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
		exitinfo := dafsm.ExecuteCurrentState() // run and get next state info
		if exitinfo.NextState == STATE_EXIT_SUCCESS {
			os.Exit(0)
		} else if exitinfo.NextState == STATE_EXIT_ERROR {
			os.Exit(1)
		} else {
			if nextstate := dafsm.GetState(exitinfo.NextState); nextstate != nil {
				dafsm.SetState(nextstate) // set next state based on info
			} else {
				log.Fatal("exiting")
				os.Exit(1)
			}
		}
	}
}

// Handle Signal in various states
func (fsm *fsm) HandleSignal(state *State, signal os.Signal) StateHandlerCode {
	/*
	* STATE Startup
	* LOOPA := STATE Hack->STATE Wait->STATE Hack->STATE Wait...
	* LOOPA until signal == SIGTERM | SIGINT | SIQUIT
	* STATE UnHack
	* END
	 */
	switch state.ID {
	case STATE_STARTUP:
		switch signal {
		case syscall.SIGTERM, syscall.SIGINT: // recv shutdown signal
			os.Exit(1) // exit before we start hacking
		}
	case STATE_HACK:
		switch signal {
		case syscall.SIGTERM, syscall.SIGINT: // recv shutdown signal
			return HANDLE_EXIT
		}
	case STATE_WAIT:
		switch signal {
		case syscall.SIGTERM, syscall.SIGINT: // recv shutdown signal
			return HANDLE_EXIT
		}
	}
	return HANDLE_CONTINUE
}
