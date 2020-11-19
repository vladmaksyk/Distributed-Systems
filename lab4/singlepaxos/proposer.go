// +build !solution

package singlepaxos

import ("dat520.github.io/lab3/detector"
)

// Proposer represents a proposer as defined by the single-decree Paxos
// algorithm.
type Proposer struct {
	crnd        Round
	clientValue Value   //cval  //do not delete
	ProcessId int
	NumOfProcesses int
	ld detector.LeaderDetector
	ClientValueIn chan Value
	PrepareOut chan<- Prepare
	PromiseIn chan Promise
	Promises []Promise
	AcceptOut chan<- Accept
	PromiseMesegesRecieved int
	PreviousSender int
	MaxValRndReported Round
	MaxRoundClientValue Value

	//TODO(student): Task 2 and 3 - algorithm and distributed implementation
	// Add other needed fields
}


// NewProposer returns a new single-decree Paxos proposer.
// It takes the following arguments:
//
// id: The id of the node running this instance of a Paxos proposer.
//
// nrOfNodes: The total number of Paxos nodes.
//
// ld: A leader detector implementing the detector.LeaderDetector interface.
//
// prepareOut: A send only channel used to send prepares to other nodes.
//
// The proposer's internal crnd field should initially be set to the value of
// its id.
func NewProposer(id int, nrOfNodes int, ld detector.LeaderDetector, prepareOut chan<- Prepare, acceptOut chan<- Accept) *Proposer {
	//TODO(student): Task 2 and 3 - algorithm and distributed implementation

	return &Proposer{
		ProcessId: id,
		NumOfProcesses: nrOfNodes,
		ld: ld,
		ClientValueIn: make(chan Value, 8),
		PrepareOut: prepareOut,
		PromiseIn : make(chan Promise, 8),
		AcceptOut: acceptOut ,
		crnd: Round(id),
		PromiseMesegesRecieved: 0,
		PreviousSender : -1,
		MaxValRndReported : Round(-1),


	}
}

// Start starts p's main run loop as a separate goroutine. The main run loop
// handles incoming promise messages and leader detector trust messages.
func (p *Proposer) Start() {
	go func() {
		for {
			select {
			case prm := <-p.PromiseIn:

				acpt, output := p.handlePromise(prm)
				if output {
					p.AcceptOut <- acpt
				}
			case ClientVal := <-p.ClientValueIn:
				leader:=p.ld.Leader()
				if p.ProcessId == leader {
					p.clientValue = ClientVal
					p.increaseCrnd()
					prep := p.sendPrepare()
					p.PrepareOut <- prep
				}else{
					p.increaseCrnd()
				}

			}
			//TODO(student): Task 3 - distributed implementation
		}
	}()
}

// Stop stops p's main run loop.
func (p *Proposer) Stop() {
	//TODO(student): Task 3 - distributed implementation
}

// DeliverPromise delivers promise prm to proposer p.
func (p *Proposer) DeliverPromise(prm Promise) {
	//TODO(student): Task 3 - distributed implementation
	p.PromiseIn <- prm
}

// DeliverClientValue delivers client value val from to proposer p.
func (p *Proposer) DeliverClientValue(val Value) {
	//TODO(student): Task 3 - distributed implementation
	p.ClientValueIn <- val
}

// Internal: handlePromise processes promise prm according to the single-decree
// Paxos algorithm. If handling the promise results in proposer p emitting a
// corresponding accept, then output will be true and acc contain the promise.
// If handlePromise returns false as output, then acc will be a zero-valued
// struct.
func (p *Proposer) handlePromise(prm Promise) (acc Accept, output bool) {
	//TODO(student): Task 2 - algorithm implementation
	quorum:=p.NumOfProcesses/2

	if prm.From != p.PreviousSender{  // count only unique senders of promiss messages
		p.PromiseMesegesRecieved++
		if prm.Vrnd > p.MaxValRndReported{ //saves the client value with the maximum Vrnd(Round in which the value was last accepted) number
			p.MaxValRndReported = prm.Vrnd
			p.MaxRoundClientValue = prm.Vval
		}
	}
	p.PreviousSender = prm.From

	if prm.Rnd == p.crnd && p.PromiseMesegesRecieved > quorum { //if valid quorum but no value reported
		if prm.Vval != ZeroValue{ // if there was a value accepted before
			p.PromiseMesegesRecieved = 0
			return Accept{From: p.ProcessId, Rnd: prm.Rnd, Val: p.MaxRoundClientValue }, true
		}
		p.PromiseMesegesRecieved = 0
		return Accept{From: p.ProcessId, Rnd: prm.Rnd, Val: p.clientValue}, true
	}else{
		return Accept{}, false
	}


}

// Internal: increaseCrnd increases proposer p's crnd field by the total number
// of Paxos nodes.
func (p *Proposer) increaseCrnd() {
	p.crnd = p.crnd + Round(p.NumOfProcesses)
	p.PromiseMesegesRecieved = 0
	//TODO(student): Task 2 - algorithm implementation
}

func (p *Proposer) sendPrepare() (prep Prepare) {
	prep1 := Prepare{From: p.ProcessId, Crnd: p.crnd}
	return prep1
}

//TODO(student): Add any other unexported methods needed.
