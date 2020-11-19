package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"dat520.github.io/lab3/detector"
	"dat520.github.io/lab4/singlepaxos"
)

// MAIN FUNCTION FOR THE SERVER
func (proc Process) handleServer(delta time.Duration) {
	// add the leader Detector
	proc.leaderDetector = detector.NewMonLeaderDetector(proc.serversIDs)
	fmt.Println("---------- ACTUAL LEADER: process n. ", proc.leaderDetector.GetLeader())
	proc.subscriber = make([]<-chan int, 1)
	for n := 0; n < len(proc.subscriber); n++ {
		proc.subscriber[n] = proc.leaderDetector.Subscribe() // our subscriber
	}

	// add the eventual failure detector
	hbOut := make(chan detector.Heartbeat, 8)
	fd := detector.NewEvtFailureDetector(proc.ourID, proc.serversIDs, proc.leaderDetector, delta, hbOut)
	proc.failureDetector = fd
	proc.failureDetector.Start()

	// ----- split here

	wg.Add(3)
	// --- paxos implementation
	proc.valueSentToClients = make(map[int]bool)
	proc.resetMapValueSentToClient()

	prepareOut := make(chan singlepaxos.Prepare, 8)
	acceptOut := make(chan singlepaxos.Accept, 8)
	promiseOut := make(chan singlepaxos.Promise, 8)
	learnOut := make(chan singlepaxos.Learn, 8)
	valueOut := make(chan singlepaxos.Value, 8)
	proc.proposer = singlepaxos.NewProposer(proc.ourID, proc.nrOfConnectedServers, proc.leaderDetector, prepareOut, acceptOut)
	proc.proposer.Start()
	proc.acceptor = singlepaxos.NewAcceptor(proc.ourID, promiseOut, learnOut)
	proc.acceptor.Start()
	proc.learner = singlepaxos.NewLearner(proc.ourID, proc.nrOfConnectedServers, valueOut)
	proc.learner.Start()

	go proc.startPaxosMethod(prepareOut, acceptOut, promiseOut, learnOut, valueOut)

	// start the goroutines to read messages and decode them
	proc.readMessages()                        // from the servers
	proc.waitForClientValueAndDeliverPrepare() // from the clients

	// create a slice of heartbeats to send to the other connected processes
	sliceHeartbeats := make([]detector.Heartbeat, 0)
	for _, id := range proc.serversIDs {
		if id != proc.ourID {
			heartbeat := detector.Heartbeat{To: id, From: proc.ourID, Request: true}
			sliceHeartbeats = append(sliceHeartbeats, heartbeat)
		}
	}
	proc.sendHeartbeats(sliceHeartbeats) // and we send them

	go proc.waitForHBSend(hbOut) // start a goroutine to wait for heartbeats response (reply)
	go proc.updateSubscribe()    // wait to update the subscriber
	wg.Wait()
}

// infinite loop for receiving value from the clients
// creates a number of goroutine equal to the number of clients
func (proc Process) waitForClientValueAndDeliverPrepare() {
	for index, conn := range proc.listeningConnsClients {
		wg.Add(1)
		go func(index int, conn net.Conn) {
			for {
				var message singlepaxos.Value
				err := json.NewDecoder(conn).Decode(&message)
				if err != nil {
					if socketIsClosed(err) {
						proc.closeTheConnection(index, conn, true)
						break
					}
				}

				fmt.Println("______RECEIVING VALUE FROM CLIENT ", index, ": ", message)

				proc.proposer.DeliverClientValue(message)
			}
			wg.Done()
		}(index, conn)
	}
}

// infinite loop to send the learned value to the clients
func (proc Process) sendValueToClients(val singlepaxos.Value) {
	for index, conn := range proc.dialingConnsClients {
		if proc.valueSentToClients[index] == false { // we didn't sent the value to this client
			err := json.NewEncoder(conn).Encode(val)
			if err != nil {
				if socketIsClosed(err) {
					proc.closeTheConnection(index, conn, true)
					return
				}
			}
			fmt.Println("SEND TO CLIENT ", index, " THE VAL: ", val)
			proc.valueSentToClients[index] = true
			time.Sleep(1000 * time.Millisecond) // wait to do something else
		}
	}
}

// MAIN FUNCTION FOR PAXOS
func (proc Process) startPaxosMethod(prepareOut chan singlepaxos.Prepare,
	acceptOut chan singlepaxos.Accept, promiseOut chan singlepaxos.Promise,
	learnOut chan singlepaxos.Learn, valueOut chan singlepaxos.Value) {
	for {
		select {
		case prp := <-prepareOut:
			// deliver the prepare
			proc.acceptor.DeliverPrepare(prp) // to myself
			// marshal (encode) the prepare
			marshPrp, _ := json.Marshal(prp)
			msg := Data{TypeOfMsg: "prepare", Data: marshPrp}
			for index := range proc.dialingConns { // and to the other processes
				if index != proc.ourID {
					proc.sendPaxosData(msg, index)
				}
			}
		case prm := <-promiseOut:
			// reset the boolean map of the valueSentToClients to false
			proc.resetMapValueSentToClient()
			proc.proposer.DeliverPromise(prm) // deliver to myself
			// marshal (encode) the promise
			marshPrm, _ := json.Marshal(prm)
			msg := Data{TypeOfMsg: "promise", Data: marshPrm}
			// receive a promise; deliver the promise to prm.To
			proc.sendPaxosData(msg, prm.To)
		case acc := <-acceptOut:
			// deliver the accept
			proc.acceptor.DeliverAccept(acc) // to myself
			// marshal (encode) the accept
			marshAcc, _ := json.Marshal(acc)
			msg := Data{TypeOfMsg: "accept", Data: marshAcc}
			for index := range proc.dialingConns { // and to the other processes
				if index != proc.ourID {
					proc.sendPaxosData(msg, index)
				}
			}
		case lrn := <-learnOut:
			// deliver the learn
			proc.learner.DeliverLearn(lrn) // to myself
			// marshal (encode) the learn
			marshLrn, _ := json.Marshal(lrn)
			msg := Data{TypeOfMsg: "learn", Data: marshLrn}
			for index := range proc.dialingConns { // and to the other processes
				if index != proc.ourID {
					proc.sendPaxosData(msg, index)
				}
			}
		case val := <-valueOut:
			// send the value to the clients
			if proc.ourID == proc.leaderDetector.Leader() {
				proc.sendValueToClients(val)
			}
		}
	}
}

func (proc Process) resetMapValueSentToClient() {
	for i := range proc.dialingConnsClients {
		proc.valueSentToClients[i] = false
	}
}
