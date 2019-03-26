// +build !solution

package singlepaxos




// Learner represents a learner as defined by the single-decree Paxos
// algorithm.
type Learner struct {
	ProcessId int
	NumOfProcesses int
	valueOut chan<- Value

	learnIn chan Learn
	stop   chan struct{}

	PreviousSender int
	LearnMesegesRecieved int
	quorumRound Round
	quorumVal Value
	SameValSameRoundCount int

	//TODO(student): Task 2 and 3 - algorithm and distributed implementation
	// Add needed fields
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
	return &Learner{
		ProcessId: id,
		NumOfProcesses: nrOfNodes,
		valueOut: valueOut,
		learnIn:   make(chan Learn, 8),
		stop:   make(chan struct{}),
		PreviousSender: -1,
		LearnMesegesRecieved : 0,
		quorumRound : Round(0),
		SameValSameRoundCount: -1,

	}
}

// Start starts l's main run loop as a separate goroutine. The main run loop
// handles incoming learn messages.
func (l *Learner) Start() {
	go func() {
		for {
			select {
			case lrn := <-l.learnIn:
				val, output := l.handleLearn(lrn)
				if output {
					l.valueOut <- val
				}
			case <-l.stop:
				return
			}
			//TODO(student): Task 3 - distributed implementation
		}
	}()
}

// Stop stops l's main run loop.
func (l *Learner) Stop() {
	l.stop <- struct{}{}
	//TODO(student): Task 3 - distributed implementation
}

// DeliverLearn delivers learn lrn to learner l.
func (l *Learner) DeliverLearn(lrn Learn) {
	l.learnIn <- lrn
	//TODO(student): Task 3 - distributed implementation
}

// Internal: handleLearn processes learn lrn according to the single-decree
// Paxos algorithm. If handling the learn results in learner l emitting a
// corresponding decided value, then output will be true and val contain the
// decided value. If handleLearn returns false as output, then val will have
// its zero value.
func (l *Learner) handleLearn(learn Learn) (val Value, output bool) {
	quorum := l.NumOfProcesses/2

	if learn.From != l.PreviousSender{  // count only unique senders of learn messages
		l.LearnMesegesRecieved++
	}
	l.PreviousSender = learn.From

	if learn.Rnd != l.quorumRound || learn.Val != l.quorumVal { // Count the amount of same Value an Round Learns
		l.SameValSameRoundCount = 0
		l.quorumRound = learn.Rnd
		l.quorumVal = learn.Val
	}
	l.SameValSameRoundCount ++


	if l.LearnMesegesRecieved > quorum && l.SameValSameRoundCount > quorum{
		l.LearnMesegesRecieved = 0
		return l.quorumVal, true
	}else {
		return ZeroValue, false
	}
	//TODO(student): Task 2 - algorithm implementation

}

//TODO(student): Add any other unexported methods needed.
