package main

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab1/detector"
	"github.com/uis-dat520-s18/glabs/grouplab2/singlepaxos"
)

type Process struct {
	ourID                 int
	nrOfProcesses         int
	nrOfConnectedServers  int // servers connected
	nodeIDs               []int
	serversIDs            []int
	listeningConns        map[int]net.Conn
	listeningConnsClients map[int]net.Conn
	dialingConns          map[int]net.Conn
	dialingConnsClients   map[int]net.Conn
	failureDetector       *detector.EvtFailureDetector
	leaderDetector        *detector.MonLeaderDetector
	subscriber            []<-chan int
	server                bool
	proposer              *singlepaxos.Proposer
	acceptor              *singlepaxos.Acceptor
	learner               *singlepaxos.Learner
	valueSentToClients    map[int]bool // value to store a boolean if we already sent the value to the client
}

var wg = sync.WaitGroup{}
var suspectChangeChan chan int
var restoreChangeChan chan int

func main() {
	delta := time.Second * 3

	proc := load_configurations()
	wg.Wait()
	fmt.Println("CONNECTIONS ESTABLISHED")

	if proc.server {
		proc.handleServer(delta)
	} else {
		proc.handleClient()
	}

}
