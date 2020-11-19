// +build !solution

package multipaxos





// Learner represents a learner as defined by the Multi-Paxos algorithm.
type Learner struct {
	ProcessId int
	NumOfProcesses int

	PreviousSender int
	LearnMesegesRecieved int
	quorumRound Round
	quorumVal Value
	SameValSameRoundCount int
	quorum int

	Vval    Value
	SlotId  SlotID


	SlotIdValue					map[SlotID]Value
	SlotIdRound   				map[SlotID]Round
	SlotIdUniqueSendersIds	map[SlotID][]int

	learnIn chan Learn
	decidedOut chan<- DecidedValue
	stop       chan struct{}

	// TODO(student)
}

// NewLearner returns a new Multi-Paxos learner. It takes the following
// arguments:
//
// id: The id of the node running this instance of a Paxos learner.
//
// nrOfNodes: The total number of Paxos nodes.
//
// decidedOut: A send only channel used to send values that has been learned,
// i.e. decided by the Paxos nodes.
func NewLearner(id int, nrOfNodes int, decidedOut chan<- DecidedValue) *Learner {
	return &Learner{
		ProcessId: id,
		NumOfProcesses: nrOfNodes,
		quorum: nrOfNodes/2 + 1,
		decidedOut: decidedOut,
		PreviousSender: -1,
		LearnMesegesRecieved : 0,
		quorumRound : Round(0),
		SameValSameRoundCount: -1,

		learnIn: make(chan Learn, 8),
		stop:    make(chan struct{}),

	}
}

// Start starts l's main run loop as a separate goroutine. The main run loop
// handles incoming learn messages.
func (l *Learner) Start() {
	go func() {
		for {
			select {
			case learn := <-l.learnIn: // receive a learn
				value, sid, output := l.handleLearn(learn)
				dValue := DecidedValue{SlotID: sid, Value: value}
				if output {
					l.decidedOut <- dValue
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
	// TODO(student)
}

// DeliverLearn delivers learn lrn to learner l.
func (l *Learner) DeliverLearn(lrn Learn) {
	l.learnIn <- lrn
	// TODO(student)
}

// Internal: handleLearn processes learn lrn according to the Multi-Paxos
// algorithm. If handling the learn results in learner l emitting a
// corresponding decided value, then output will be true, sid the id for the
// slot that was decided and val contain the decided value. If handleLearn
// returns false as output, then val and sid will have their zero value.
func (l *Learner) handleLearn(learn Learn) (val Value, sid SlotID, output bool) {
	output = false

	if l.SlotIdValue == nil { // if it's empty, initialize the map of value round slots
		l.SlotIdValue = make(map[SlotID]Value)
	}
	if l.SlotIdRound == nil { // if it's empty, initialize the map of value round slots
		l.SlotIdRound = make(map[SlotID]Round)
	}
	if l.SlotIdUniqueSendersIds == nil { // if it's empty, initialize the map of value round slots
		l.SlotIdUniqueSendersIds = make(map[SlotID][]int)
	}

	// fmt.Println("learn : ", learn)


	if learn.Rnd >= l.SlotIdRound[learn.Slot] {
		if learn.Rnd > l.SlotIdRound[learn.Slot] {
			l.SlotIdUniqueSendersIds[learn.Slot] = nil
		}
		IdPresence := Contains(l.SlotIdUniqueSendersIds[learn.Slot], learn.From)  //Check if this sender is already present in the slots recors of senders
		if !IdPresence{ // only ad the sender id to the slots record list of senders if the sender is not already present in that list
			l.SlotIdUniqueSendersIds[learn.Slot] = append(l.SlotIdUniqueSendersIds[learn.Slot], learn.From)
		}
		l.SlotIdRound[learn.Slot] = learn.Rnd
		l.SlotIdValue[learn.Slot] = learn.Val
	}

	if len(l.SlotIdUniqueSendersIds[learn.Slot]) >= l.quorum{ // if a slot reached a quorum of senders output its current number and Value
		output = true
		l.Vval = l.SlotIdValue[learn.Slot]
		l.SlotId = learn.Slot

		l.SlotIdUniqueSendersIds[learn.Slot] = nil //after writing the output set the unique senders to 0 again
	}


	return l.Vval, l.SlotId, output
}

func Contains(s []int, e int) bool {
    for _, a := range s {
        if a == e {
            return true
        }
    }
    return false
}

func ContainsInt(s int, e int) bool {
     if s == e {
         return true
    }
    return false
}
