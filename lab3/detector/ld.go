// +build !solution

package detector

// A MonLeaderDetector represents a Monarchical Eventual Leader Detector as
// described at page 53 in:
// Christian Cachin, Rachid Guerraoui, and Luís Rodrigues: "Introduction to
// Reliable and Secure Distributed Programming" Springer, 2nd edition, 2011.
type MonLeaderDetector struct {
	// TODO(student): Add needed fields
	alive         map[int]bool // map of node ids considered alive
	suspected     map[int]bool // map of node ids  considered suspected
	nodeIds       []int
	Channels      []chan int
	Channel       chan int
	CurrentLeader int
	LeaderChange  bool
	Allsuspected  bool
}

// NewMonLeaderDetector returns a new Monarchical Eventual Leader Detector
// given a list of node ids.
func NewMonLeaderDetector(nodeIDs []int) *MonLeaderDetector {
	alive := make(map[int]bool)
	suspected := make(map[int]bool)
	channels := make([]chan int, 0, 10)
	channel := make(chan int, 1)

	for i := 0; i < len(nodeIDs); i++ {
		alive[nodeIDs[i]] = true // set all nodes alive
	}

	return &MonLeaderDetector{
		alive:        alive,
		suspected:    suspected,
		nodeIds:      nodeIDs,
		Channels:     channels,
		Channel:      channel,
		LeaderChange: false,
		Allsuspected: false,
	}
}

// Leader returns the current leader. Leader will return UnknownID if all nodes
// are suspected.
func (m *MonLeaderDetector) Leader() int {
	// TODO(student): Implement
	if len(m.alive) == 0 {
		m.Allsuspected = true
		return UnknownID
	}

	leader := -1
	for key := range m.alive { //elect the node with highest id as a leader
		if key > leader {
			leader = key
		}
	}

	if leader < 0 { // if all suspected
		return UnknownID
	}

	if m.CurrentLeader != leader { // if the leader changed
		m.LeaderChange = true
	} else {
		m.LeaderChange = false
	}

	m.CurrentLeader = leader //save the current elected leader
	return leader

}

// Suspect instructs m to consider the node with matching id as suspected. If
// the suspect indication result in a leader change the leader detector should
// publish this change to its subscribers.
func (m *MonLeaderDetector) Suspect(id int) {
	// TODO(student): Implement
	for j := range m.alive {
		if id == j {
			m.suspected[id] = true
			delete(m.alive, id)
		}
	}

	//Publish to subscribers
	newLeader := m.Leader() // change the leader if some node was suspected
	var i int
	if m.LeaderChange || m.Allsuspected {
		for i < len(m.Channels) {
			m.Channels[i] <- newLeader
			i++
		}
	}

}

// Restore instructs m to consider the node with matching id as restored. If
// the restore indication result in a leader change the leader detector should
// publish this change to its subscribers.
func (m *MonLeaderDetector) Restore(id int) {
	// TODO(student): Implement
	_, ok := m.suspected[id]
	if ok == true {
		delete(m.suspected, id)
		m.alive[id] = true
	}

	//Publish to subscribers
	var j int
	newLeader := m.Leader()
	if m.LeaderChange || m.Allsuspected {
		for j < len(m.Channels) {
			m.Channels[j] <- newLeader
			j++
		}
	}
}

// Subscribe returns a buffered channel, that m on leader change will use to
// publish the id of the highest ranking node.
//
// The leader detector will publish UnknownID if all nodes become suspected.
// Subscribe will drop publications to slow subscribers.
// Note: Subscribe returns a unique channel to every
// subscriber; it is not meant to be shared.
func (m *MonLeaderDetector) Subscribe() <-chan int {
	subscriber := make(chan int, 1)
	m.Channels = append(m.Channels, subscriber)
	return subscriber
}

// TODO(student): Add other unexported functions or methods if needed.
