// +build !solution

package multipaxos

import "sort"

// Acceptor represents an acceptor as defined by the Multi-Paxos algorithm.
type Acceptor struct {
	HighestRoundSeen Round
	ProcessId         int
	slot_vval  map[SlotID]Value
	slot_vrnd  map[SlotID]Round

	prepareIn  chan Prepare
	promiseOut chan<- Promise
	acceptIn   chan Accept
	learnOut   chan<- Learn
	stop       chan struct{}

}

// NewAcceptor returns a new Multi-Paxos acceptor. It takes the following
// arguments:
//
// id: The id of the node running this instance of a Paxos acceptor.
//
// promiseOut: A send only channel used to send promises to other nodes.
//
// learnOut: A send only channel used to send learns to other nodes.
func NewAcceptor(id int, promiseOut chan<- Promise, learnOut chan<- Learn) *Acceptor {
	return &Acceptor{
		HighestRoundSeen : -1,
		ProcessId :  id,

		promiseOut: promiseOut,
		learnOut:   learnOut,
		prepareIn:  make(chan Prepare, 10),
		acceptIn:   make(chan Accept, 10),
		stop:       make(chan struct{}),


	}
}

// Start starts a's main run loop as a separate goroutine. The main run loop
// handles incoming prepare and accept messages.
func (a *Acceptor) Start() {
	go func() {
		for {
			select {
			case prp := <-a.prepareIn: // receive a prepare
				prm, output := a.handlePrepare(prp)
				if output {
					a.promiseOut <- prm
				}
			case acc := <-a.acceptIn: // receive an accept
				learn, output := a.handleAccept(acc)
				if output {
					a.learnOut <- learn
				}
			case <-a.stop:
				return
			}

		}
	}()
}

// Stop stops a's main run loop.
func (a *Acceptor) Stop() {
	a.stop <- struct{}{}
	// TODO(student)
}

// DeliverPrepare delivers prepare prp to acceptor a.
func (a *Acceptor) DeliverPrepare(prp Prepare) {
	a.prepareIn <- prp
	// TODO(student)
}

// DeliverAccept delivers accept acc to acceptor a.
func (a *Acceptor) DeliverAccept(acc Accept) {
	a.acceptIn <- acc
	// TODO(student)
}

// Internal: handlePrepare processes prepare prp according to the Multi-Paxos
// algorithm. If handling the prepare results in acceptor a emitting a
// corresponding promise, then output will be true and prm contain the promise.
// If handlePrepare returns false as output, then prm will be a zero-valued
// struct.
func (a *Acceptor) handlePrepare(prp Prepare) (prm Promise, output bool) {
	output = false

	if prp.Crnd >= a.HighestRoundSeen{
		a.HighestRoundSeen = prp.Crnd
		output = true
		prm = Promise{To: prp.From, From: a.ProcessId, Rnd: a.HighestRoundSeen, Slots: []PromiseSlot{}}

		var slotsList []int
		for k := range a.slot_vrnd {// sort in increasing order
			slotsList = append(slotsList, int(k))
		}
		sort.Ints(slotsList)
		for _, SlotId := range slotsList{  // sort the Slots in increasing order
			if SlotId >= int(prp.Slot) {
				prm.Slots = append(prm.Slots, PromiseSlot{ID : SlotID(SlotId) , Vrnd: a.slot_vrnd[SlotID(SlotId)], Vval: a.slot_vval[SlotID(SlotId)] } )
			}
		}

		if len(prm.Slots) == 0{  // if there are no slots then return nil
			prm.Slots = nil
		}
	}

	return prm, output
}

// Internal: handleAccept processes accept acc according to the Multi-Paxos
// algorithm. If handling the accept results in acceptor a emitting a
// corresponding learn, then output will be true and lrn contain the learn.  If
// handleAccept returns false as output, then lrn will be a zero-valued struct.
func (a *Acceptor) handleAccept(acc Accept) (lrn Learn, output bool) {
	output = false

	if a.slot_vrnd == nil { // if it's empty, initialize the map of value round slots
		a.slot_vrnd = make(map[SlotID]Round)
	}
	if a.slot_vval == nil { // if it's empty, initialize the map of vval slots
		a.slot_vval = make(map[SlotID]Value)
	}

	if acc.Rnd >= a.HighestRoundSeen && acc.Rnd != a.slot_vrnd[acc.Slot]{

		output = true
		a.HighestRoundSeen = acc.Rnd

		// save value and vround for promise slots
		a.slot_vrnd[acc.Slot] = acc.Rnd
		a.slot_vval[acc.Slot] = acc.Val

	}
	return Learn{ From: a.ProcessId, Slot: acc.Slot, Rnd: a.HighestRoundSeen, Val: a.slot_vval[acc.Slot] }, output
}






//
