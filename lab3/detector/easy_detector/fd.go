// +build !solution

//package detector
package main

import (
	"fmt"
	"time"
	"sync"
)

var wg = sync.WaitGroup{}

type Heartbeat struct {
	From    int
	To      int
	Request bool // true -> request, false -> reply
}

type SuspectRestorer interface {
	Suspecter
	Restorer
}
type Suspecter interface {
	Suspect(id int)
}

type Restorer interface {
	Restore(id int)
}
type Accumulator struct {
	Suspects map[int]bool
	Restores map[int]bool
}

func NewAccumulator() *Accumulator {
	return &Accumulator{
		Suspects: make(map[int]bool),
		Restores: make(map[int]bool),
	}
}

func (a *Accumulator) Suspect(id int) {
	a.Suspects[id] = true
}

func (a *Accumulator) Restore(id int) {
	a.Restores[id] = true
}

func (a *Accumulator) Reset() {
	a.Suspects = make(map[int]bool)
	a.Restores = make(map[int]bool)
}

type EvtFailureDetector struct {
	id        int          // this node's id
	nodeIDs   []int        // node ids for every node in cluster
	alive     map[int]bool // map of node ids considered alive
	suspected map[int]bool // map of node ids considered suspected

	sr SuspectRestorer // Provided SuspectRestorer implementation

	delay         time.Duration // the current delay for the timeout procedure
	delta         time.Duration // the delta value to be used when increasing delay
	timeoutSignal *time.Ticker  // the timeout procedure ticker

	hbSend chan<- Heartbeat // channel for sending outgoing heartbeat messages
	hbIn   chan Heartbeat   // channel for receiving incoming heartbeat messages
	stop   chan struct{}    // channel for signaling a stop request to the main run loop

	testingHook func() // DO NOT REMOVE THIS LINE. A no-op when not testing.
}

const (
	ourID = 2
	delta = time.Second
)

var clusterOfThree = []int{2, 1, 0}


func NewEvtFailureDetector(id int, nodeIDs []int, sr SuspectRestorer, delta time.Duration, hbSend chan<- Heartbeat) *EvtFailureDetector {
	suspected := make(map[int]bool)
	alive := make(map[int]bool)

	LenString := len(nodeIDs)
	for i := 0; i < LenString; i++ { //1. set all nodes alive
		alive[i] = true
	}


	return &EvtFailureDetector{
		id:        id,
		nodeIDs:   nodeIDs,
		alive:     alive,
		suspected: suspected,

		sr: sr, //suspect restore object

		delay: delta,
		delta: delta,

		hbSend: hbSend,
		hbIn:   make(chan Heartbeat, 8),
		stop:   make(chan struct{}),

		testingHook: func() {}, // DO NOT REMOVE THIS LINE. A no-op when not testing.
	}
}


func (e *EvtFailureDetector) Start() {
	e.timeoutSignal = time.NewTicker(e.delay)
	go func() {
		for {
			e.testingHook() // DO NOT REMOVE THIS LINE. A no-op when not testing.
			select {
			case hb := <-e.hbIn:
				hbReply := Heartbeat{To: hb.From, From: hb.To, Request: false}

				if hb.Request == true { // if this is a hb request sent hb reply
					e.hbSend <- hbReply

				}else { // if this is a reply ,mark the node as alive
					fmt.Println("inside")
					if e.alive[hb.From] != true {
						e.alive[hb.From] = true
					}
				}

				// TODO(student): Handle incoming heartbeat
			case <-e.timeoutSignal.C:
				e.timeout()
			case <-e.stop:
				return
			}
		}
	}()
}

// DeliverHeartbeat delivers heartbeat hb to failure detector e.
func (e *EvtFailureDetector) DeliverHeartbeat(hb Heartbeat) {
	e.hbIn <- hb
}

// Stop stops e's main run loop.
func (e *EvtFailureDetector) Stop() {
	e.stop <- struct{}{}
}

// Internal: timeout runs e's timeout procedure.
func (e *EvtFailureDetector) timeout() {
	// TODO(student): Implement timeout procedure

	if len(e.alive) != 0 && len(e.suspected) != 0 { //if the node is poth susp and alive invcesase timout by delta
		//fmt.Println("alive : ", e.alive, "suspected : ", e.suspected)
		for j := 0; j < len(e.nodeIDs); j++ {
			if e.alive[e.nodeIDs[j]] == e.suspected[e.nodeIDs[j]] {
				e.delay += e.delta
				break
			}
		}
	}


	for i := 0; i < len(e.nodeIDs); i++ { // if both are abcsent in alive and suspect, or both are present in alive and suspect
		flag := 0
		for k := range e.alive {
			if e.nodeIDs[i] == k {
				flag++
				break
			}
		}
		for l := range e.suspected {
			if e.nodeIDs[i] == l {
				flag++
				break
			}
		}

		if flag == 0 {
			e.suspected[e.nodeIDs[i]] = true
			e.sr.Suspect(e.nodeIDs[i])
		} else if flag == 2 {
			delete(e.suspected, e.nodeIDs[i])
			e.sr.Restore(e.nodeIDs[i])
		}
	}

	for i := 0; i < len(e.nodeIDs); i++ {
		hbRequest := Heartbeat{To: e.nodeIDs[i], From: e.id, Request: true}
		e.hbSend <- hbRequest
	}

	for i := range e.alive { // making alive set empty
		delete(e.alive, i)
	}
	e.timeoutSignal = time.NewTicker(e.delay)
}

var timeoutTests = []struct {
	alive             map[int]bool
	suspected         map[int]bool
	wantPostSuspected map[int]bool
	wantSuspects      []int
	wantRestores      []int
	wantDelay         time.Duration
}{
	{
		map[int]bool{2: true, 1: true, 0: true},
		map[int]bool{},
		map[int]bool{},
		[]int{},
		[]int{},
		delta,
	},
}

func main(){

	fmt.Println("heyyy")
	acc := NewAccumulator()
	hbOut := make(chan Heartbeat, 16)
	fd := NewEvtFailureDetector(ourID, clusterOfThree, acc, delta, hbOut)
	fmt.Println(fd)
	fd.Start()


	for i := 0; i < 3; i++ {
		hbReq := Heartbeat{To: ourID, From: i, Request: true}
		fd.DeliverHeartbeat(hbReq)
		reply := <-hbOut
		fmt.Println(reply)
	}

	fd.alive[0] = false
	fmt.Println("aliveset:",fd.alive)
	fmt.Println("---------NEXT")


	for i := 0; i < 3; i++ {
		hbReq := Heartbeat{To: ourID, From: i, Request: false}
		fd.DeliverHeartbeat(hbReq)
	}
	time.Sleep(500 * time.Millisecond)
	fmt.Println("aliveset:",fd.alive)

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

}



// TODO(student): Add other unexported functions or methods if needed.
