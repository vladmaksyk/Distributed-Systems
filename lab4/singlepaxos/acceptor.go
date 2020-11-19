// +build !solution

package singlepaxos



// Acceptor represents an acceptor as defined by the single-decree Paxos
// algorithm.
type Acceptor struct {
	PrepareIn chan Prepare
	HighRoundSeen Round
	Vrnd Round

	AcceptIn chan Accept
	Vval Value

	ProcessId int
	PromiseOut chan <- Promise
	LearnOut chan <- Learn

	stop   chan struct{}

	//TODO(student): Task 2 and 3 - algorithm and distributed implementation
	// Add needed fields
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
		PrepareIn:   make(chan Prepare, 8),
		HighRoundSeen: 0,
		Vrnd: 0,

		AcceptIn:   make(chan Accept, 8),
		Vval: "",

		ProcessId: id,
		PromiseOut: promiseOut,
		LearnOut: learnOut,
		stop:   make(chan struct{}),

	}
}

// Start starts a's main run loop as a separate goroutine. The main run loop
// handles incoming prepare and accept messages.
func (a *Acceptor) Start() {
	go func() {
		for {
			select {
			case prp := <-a.PrepareIn:
				prm, output := a.handlePrepare(prp)
				if output {
					a.PromiseOut <- prm
				}
			case acpt := <-a.AcceptIn:
				lrn, output := a.handleAccept(acpt)
				if output {
					a.LearnOut <- lrn
				}
			case <-a.stop:
				return
			}
			//TODO(student): Task 3 - distributed implementation
		}
	}()
}

// Stop stops a's main run loop.
func (a *Acceptor) Stop() {
	a.stop <- struct{}{}
	//TODO(student): Task 3 - distributed implementation
}

// DeliverPrepare delivers prepare prp to acceptor a.
func (a *Acceptor) DeliverPrepare(prp Prepare) {
	a.PrepareIn <- prp
	//TODO(student): Task 3 - distributed implementation
}

// DeliverAccept delivers accept acc to acceptor a.
func (a *Acceptor) DeliverAccept(acc Accept) {
	a.AcceptIn <- acc
	//TODO(student): Task 3 - distributed implementation
}

// Internal: handlePrepare processes prepare prp according to the single-decree
// Paxos algorithm. If handling the prepare results in acceptor a emitting a
// corresponding promise, then output will be true and prm contain the promise.
// If handlePrepare returns false as output, then prm will be a zero-valued
// struct.
func (a *Acceptor) handlePrepare(prp Prepare) (prm Promise, output bool) {
	//TODO(student): Task 2 - algorithm implementation

	if a.HighRoundSeen == 0 { //means that there were no prepare messages before
		a.HighRoundSeen = prp.Crnd
		return Promise{To: prp.From, From: a.ProcessId, Rnd: prp.Crnd, Vrnd: NoRound, Vval: ZeroValue}, true
	}

	if prp.Crnd > a.HighRoundSeen { // if the round is bigger then just reply with that bigger round
		a.HighRoundSeen = prp.Crnd
		if a.Vrnd == 0{  //if there were no values accepted before
			return Promise{To: prp.From, From: a.ProcessId, Rnd: prp.Crnd, Vrnd: NoRound, Vval: ZeroValue}, true
		}else{
			return Promise{To: prp.From, From: a.ProcessId, Rnd: prp.Crnd, Vrnd: a.Vrnd, Vval: a.Vval}, true
		}
	}else{
		return Promise{}, false
	}
}

// Internal: handleAccept processes accept acc according to the single-decree
// Paxos algorithm. If handling the accept results in acceptor a emitting a
// corresponding learn, then output will be true and lrn contain the learn.  If
// handleAccept returns false as output, then lrn will be a zero-valued struct.
func (a *Acceptor) handleAccept(acc Accept) (lrn Learn, output bool) {
	if acc.Rnd >= a.HighRoundSeen{ // accept the accept message only if the round is equal or bigger than the highest round seen
		a.HighRoundSeen = acc.Rnd

		a.Vrnd=acc.Rnd
		a.Vval = acc.Val

		return Learn{From: a.ProcessId, Rnd: a.HighRoundSeen, Val: a.Vval}, true
	}else{
		return Learn{}, false
	}
	//TODO(student): Task 2 - algorithm implementation

}

//TODO(student): Add any other unexported methods needed.
