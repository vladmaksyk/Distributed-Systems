// +build !solution

package singlepaxos

import "fmt"

// Acceptor represents an acceptor as defined by the single-decree Paxos
// algorithm.
type Acceptor struct {
	id         int
	rnd        Round
	vval       Value
	vrnd       Round
	prepareIn  chan Prepare
	promiseOut chan<- Promise
	acceptIn   chan Accept
	learnOut   chan<- Learn
	stop       chan struct{}
}

// NewAcceptor returns a new single-decree Paxos acceptor.
// It takes the following arguments:
//
// id: The id of the node running this instance of a Paxos acceptor.
//
// promiseOut: A send only channel used to send promises to other nodes.
//
// learnOut: A send only channel used to send learns to other nodes.
func NewAcceptor(id int, promiseOut chan<- Promise, learnOut chan<- Learn) *Acceptor {
	//TODO(student): Task 2 and 3 - algorithm and distributed implementation
	return &Acceptor{
		id:         id,
		rnd:        NoRound,
		vval:       ZeroValue,
		vrnd:       NoRound,
		promiseOut: promiseOut,
		learnOut:   learnOut,
		prepareIn:  make(chan Prepare, 8),
		acceptIn:   make(chan Accept, 8),
		stop:       make(chan struct{}),
	}
}

// Start starts a's main run loop as a separate goroutine. The main run loop
// handles incoming prepare and accept messages.
func (a *Acceptor) Start() {
	go func() {
		for {
			select {
			case prp := <-a.prepareIn:
				prm, output := a.handlePrepare(prp)
				if output {
					fmt.Println("<-CREATING", prm)
					a.promiseOut <- prm
				}
			case acc := <-a.acceptIn:
				learn, output := a.handleAccept(acc)
				if output {
					fmt.Println("<-CREATING", learn)
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
}

// DeliverPrepare delivers prepare prp to acceptor a.
func (a *Acceptor) DeliverPrepare(prp Prepare) {
	a.prepareIn <- prp
}

// DeliverAccept delivers accept acc to acceptor a.
func (a *Acceptor) DeliverAccept(acc Accept) {
	a.acceptIn <- acc
}

// Internal: handlePrepare processes prepare prp according to the single-decree
// Paxos algorithm. If handling the prepare results in acceptor a emitting a
// corresponding promise, then output will be true and prm contain the promise.
// If handlePrepare returns false as output, then prm will be a zero-valued
// struct.
func (a *Acceptor) handlePrepare(prp Prepare) (prm Promise, output bool) {
	output = false
	if prp.Crnd > a.rnd {
		a.rnd = prp.Crnd
		output = true
	}
	return Promise{To: prp.From, From: a.id, Rnd: a.rnd, Vrnd: a.vrnd, Vval: a.vval}, output
}

// Internal: handleAccept processes accept acc according to the single-decree
// Paxos algorithm. If handling the accept results in acceptor a emitting a
// corresponding learn, then output will be true and lrn contain the learn.  If
// handleAccept returns false as output, then lrn will be a zero-valued struct.
func (a *Acceptor) handleAccept(acc Accept) (lrn Learn, output bool) {
	output = false
	if acc.Rnd >= a.rnd && acc.Rnd != a.vrnd {
		a.rnd = acc.Rnd
		a.vrnd = acc.Rnd
		a.vval = acc.Val
		output = true
	}
	return Learn{From: a.id, Rnd: a.rnd, Val: a.vval}, output
}
