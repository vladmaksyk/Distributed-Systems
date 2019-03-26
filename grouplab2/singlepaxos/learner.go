// +build !solution

package singlepaxos

import "fmt"

// Learner represents a learner as defined by the single-decree Paxos
// algorithm.
type Learner struct {
	id         int
	nrOfNodes  int
	vval       Value
	learnIn    chan Learn
	setOfNodes map[int]Learn
	valueOut   chan<- Value
	stop       chan struct{}
}

// NewLearner returns a new single-decree Paxos learner. It takes the
// following arguments:
//
// id: The id of the node running this instance of a Paxos learner.
//
// nrOfNodes: The total number of Paxos nodes.
//
// valueOut: A send only channel used to send values that has been learned,
// i.e. decided by the Paxos nodes.
func NewLearner(id int, nrOfNodes int, valueOut chan<- Value) *Learner {
	//TODO(student): Task 2 and 3 - algorithm and distributed implementation
	setOfNodes := make(map[int]Learn)

	for node := 0; node < nrOfNodes; node++ {
		setOfNodes[node] = Learn{}
	}

	return &Learner{
		id:         id,
		nrOfNodes:  nrOfNodes,
		setOfNodes: setOfNodes,
		valueOut:   valueOut,
		learnIn:    make(chan Learn, 8),
		stop:       make(chan struct{}),
	}
}

// Start starts l's main run loop as a separate goroutine. The main run loop
// handles incoming learn messages.
func (l *Learner) Start() {
	go func() {
		for {
			select {
			case learn := <-l.learnIn:
				value, output := l.handleLearn(learn)
				if output {
					fmt.Println("<-CREATING", value)
					l.valueOut <- value
				}
			case <-l.stop:
				return
			}
		}
	}()
}

// Stop stops l's main run loop.
func (l *Learner) Stop() {
	l.stop <- struct{}{}
}

// DeliverLearn delivers learn lrn to learner l.
func (l *Learner) DeliverLearn(lrn Learn) {
	l.learnIn <- lrn
}

// Internal: handleLearn processes learn lrn according to the single-decree
// Paxos algorithm. If handling the learn results in learner l emitting a
// corresponding decided value, then output will be true and val contain the
// decided value. If handleLearn returns false as output, then val will have
// its zero value.
func (l *Learner) handleLearn(learn Learn) (val Value, output bool) {
	l.setOfNodes[learn.From] = learn
	tmpMapValue := make(map[int]Value)
	quorum := l.nrOfNodes / 2
	output = false

	for i, node := range l.setOfNodes {
		if learn.Rnd == node.Rnd {
			tmpMapValue[i] = node.Val
			l.vval = node.Val
		}
	}
	if len(tmpMapValue) > quorum {
		output = true
	}
	return l.vval, output
}
