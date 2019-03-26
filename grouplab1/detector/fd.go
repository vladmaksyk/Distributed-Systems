// +build !solution

package detector

import (

	"time"
)

// EvtFailureDetector represents a Eventually Perfect Failure Detector as
// described at page 53 in:
// Christian Cachin, Rachid Guerraoui, and Luís Rodrigues: "Introduction to
// Reliable and Secure Distributed Programming" Springer, 2nd edition, 2011.
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

// NewEvtFailureDetector returns a new Eventual Failure Detector. It takes the
// following arguments:
//
// id: The id of the node running this instance of the failure detector.
//
// nodeIDs: A list of ids for every node in the cluster (including the node
// running this instance of the failure detector).
//
// ld: A leader detector implementing the SuspectRestorer interface.
//
// delta: The initial value for the timeout interval. Also the value to be used
// when increasing delay.
//
// hbSend: A send only channel used to send heartbeats to other nodes.
func NewEvtFailureDetector(id int, nodeIDs []int, sr SuspectRestorer, delta time.Duration, hbSend chan<- Heartbeat) *EvtFailureDetector {
	suspected := make(map[int]bool)
	alive := make(map[int]bool)

	LenString := len(nodeIDs)
	for i := 0; i < LenString; i++ { //1. set all nodes alive
		alive[i] = true
	}

	// TODO(student): perform any initialization necessary

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

// Start starts e's main run loop as a separate goroutine. The main run loop
// handles incoming heartbeat requests and responses. The loop also trigger e's
// timeout procedure at an interval corresponding to e's internal delay
// duration variable.
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
				} else if hb.Request != true { // if this is a reply ,ark the node as alive
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

	//fmt.Println("alive : ", e.alive, "suspected : ", e.suspected, "nodeIDs :", e.nodeIDs)

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

	// for i := 0; i < len(e.nodeIDs); i++ {
	// 	hbRequest := Heartbeat{To: e.nodeIDs[i], From: e.id, Request: true}
	// 	e.hbSend <- hbRequest
	// }

	for i := range e.alive { // making alive set empty
		delete(e.alive, i)
	}
	e.timeoutSignal = time.NewTicker(e.delay)
}

// TODO(student): Add other unexported functions or methods if needed.
