// +build !solution

package main
import (
"fmt")


const UnknownID int = -1
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
		for j:=0; j < len(m.Channels);j++ {
			m.Channels[j] <- newLeader
		
		}
	}
}


func (m *MonLeaderDetector) Subscribe() <-chan int {
	subscriber := make(chan int, 1)
	m.Channels = append(m.Channels, subscriber)
	return subscriber
}



func main(){
	AllProcessIDs := []int{0,1,2,5,6,7}
	fmt.Println("hiii")
	ld := NewMonLeaderDetector(AllProcessIDs)
	gotLeader := ld.Leader()
	fmt.Println(gotLeader)
	ld.Suspect(AllProcessIDs[5])
	gotLeader = ld.Leader()
	fmt.Println(gotLeader)

}

// TODO(student): Add other unexported functions or methods if needed.
