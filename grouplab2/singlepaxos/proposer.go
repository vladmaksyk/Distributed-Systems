// +build !solution

package singlepaxos

import (
	"fmt"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab1/detector"
)

// Proposer represents a proposer as defined by the single-decree Paxos
// algorithm.
type Proposer struct {
	id                    int
	nrOfNodes             int
	mapOfPromisesPerRound map[int]map[int]Promise
	crnd                  Round
	clientValue           Value
	ld                    detector.LeaderDetector
	promiseIn             chan Promise
	prepareOut            chan<- Prepare
	acceptOut             chan<- Accept
	timeoutSignal         *time.Ticker // the timeout procedure ticker
	valueArrived          bool
	stop                  chan struct{}
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
	mapOfPromisesPerRound := make(map[int]map[int]Promise)

	return &Proposer{
		id:        id,
		nrOfNodes: nrOfNodes,
		crnd:      Round(id),
		mapOfPromisesPerRound: mapOfPromisesPerRound,
		ld:           ld,
		prepareOut:   prepareOut,
		acceptOut:    acceptOut,
		promiseIn:    make(chan Promise, 8),
		valueArrived: false,
		stop:         make(chan struct{}),
	}
}

// Start starts p's main run loop as a separate goroutine. The main run loop
// handles incoming promise messages and leader detector trust messages.
func (p *Proposer) Start() {
	p.timeoutSignal = time.NewTicker(time.Second * 5)
	go func() {
		for {
			select {
			case prm := <-p.promiseIn:
				acc, output := p.handlePromise(prm)
				if output {
					fmt.Println("<-CREATING", acc)
					p.valueArrived = false // set to false because we reached the quorum and send the accept
					p.acceptOut <- acc
				}
			case <-p.timeoutSignal.C:
				p.timeout()
			case <-p.stop:
				return
			}
		}
	}()
}

// Stop stops p's main run loop.
func (p *Proposer) Stop() {
	p.stop <- struct{}{}
}

// DeliverPromise delivers promise prm to proposer p.
func (p *Proposer) DeliverPromise(prm Promise) {
	p.promiseIn <- prm
}

// DeliverClientValue delivers client value val from to proposer p.
func (p *Proposer) DeliverClientValue(val Value) {
	if p.id == p.ld.Leader() {
		if p.clientValue == ZeroValue {
			if val != ZeroValue {
				p.clientValue = val
				p.valueArrived = true
			}
		} else {
			// clientValue already decided (SINCE WE ARE IN A SINGLE PAXOS!!!)
			fmt.Println("Value already decided:", p.clientValue, "; send it back to the client")
		}
		p.increaseCrnd()
		prp := Prepare{From: p.id, Crnd: p.crnd}
		fmt.Println("<-CREATING", prp)
		p.prepareOut <- prp
	} else {
		fmt.Println("I'm not the leader, discard the client value...")
	}

}

// Internal: handlePromise processes promise prm according to the single-decree
// Paxos algorithm. If handling the promise results in proposer p emitting a
// corresponding accept, then output will be true and acc contain the promise.
// If handlePromise returns false as output, then acc will be a zero-valued
// struct.
func (p *Proposer) handlePromise(prm Promise) (acc Accept, output bool) {
	quorum := p.nrOfNodes / 2
	output = false

	if len(p.mapOfPromisesPerRound[int(prm.Rnd)]) == 0 {
		p.mapOfPromisesPerRound[int(prm.Rnd)] = make(map[int]Promise)
	}

	p.mapOfPromisesPerRound[int(prm.Rnd)][prm.From] = prm

	// only if we exceed the quorum for the promise in our current round!
	if len(p.mapOfPromisesPerRound[int(p.crnd)]) > quorum {
		p.clientValue = p.safeValue(p.mapOfPromisesPerRound[int(prm.Rnd)])
		output = true
	}

	return Accept{From: p.id, Rnd: p.crnd, Val: p.clientValue}, output
}

// Internal: increaseCrnd increases proposer p's crnd field by the total number
// of Paxos nodes.
func (p *Proposer) increaseCrnd() {
	p.crnd += Round(p.nrOfNodes)
	p.mapOfPromisesPerRound[int(p.crnd)] = make(map[int]Promise) // empty the map for the new current round
}

func (p *Proposer) safeValue(promises map[int]Promise) (val Value) {
	var (
		tmpMaxRnd Round
		tmpValue  Value
	)
	tmpMaxRnd = 0
	for _, prom := range promises {
		if tmpMaxRnd == 0 && prom.Vval != ZeroValue {
			tmpValue = prom.Vval
		}

		if prom.Vrnd > tmpMaxRnd {
			tmpMaxRnd = prom.Vrnd
			tmpValue = prom.Vval
		}

		if tmpValue == ZeroValue {
			tmpValue = p.clientValue
		}
	}
	return tmpValue
}

func (p *Proposer) timeout() {
	// only if we already received the value but not the promise, we increase the crnd and send a new prepare
	if p.valueArrived {
		p.increaseCrnd()
		p.prepareOut <- Prepare{From: p.id, Crnd: p.crnd}
	}
	p.timeoutSignal = time.NewTicker(time.Second * 5)
}
