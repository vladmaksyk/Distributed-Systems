package main

import (
	"fmt"
	"net"
	"sync"
	"time"
	"strconv"
	"strings"
	"encoding/json"
	"dat520.github.io/lab3/detector"
	"dat520.github.io/lab4/singlepaxos"
)

type IncomingInitialMessage struct {
	ProcId int
	Server bool
}

type socket struct {
	address string
	server  bool
}

type Data struct {
	TypeOfMsg string
	Data      []byte
	// Data interface{}
}

var listeningSockStrings = []socket{
	socket{address: "localhost:12100", server: true},
	socket{address: "localhost:12101", server: true},
	socket{address: "localhost:12102", server: true},
	socket{address: "localhost:12103", server: false},
	socket{address: "localhost:12104", server: false},
}

type Process struct {
	ourID                 int
	nrOfProcesses         int
	nrOfConnectedServers  int // servers connected
	nodeIDs               []int
	serversIDs            []int
	listeningConns        map[int]net.Conn
	listeningConnsClients map[int]net.Conn
	dialingConns          map[int]net.Conn
	dialingConnsClients   map[int]net.Conn
	failureDetector       *detector.EvtFailureDetector
	leaderDetector        *detector.MonLeaderDetector
	subscriber            []<-chan int
	server                bool
	proposer              *singlepaxos.Proposer
	acceptor              *singlepaxos.Acceptor
	learner               *singlepaxos.Learner
	valueSentToClients    map[int]bool // value to store a boolean if we already sent the value to the client
}

var wg = sync.WaitGroup{}
var suspectChangeChan chan int
var restoreChangeChan chan int

func load_configurations() *Process {
	proc := Process{}

	var err error
	var acceptErr error
	proc.nrOfConnectedServers = 0
	proc.nrOfProcesses = 5
	proc.dialingConns = make(map[int]net.Conn)
	proc.listeningConns = make(map[int]net.Conn)
	proc.dialingConnsClients = make(map[int]net.Conn)
	proc.listeningConnsClients = make(map[int]net.Conn)

	for {
		proc.ourID = get_int_from_terminal("Specify the ID of the process")
		if proc.ourID < 0 || proc.ourID > proc.nrOfProcesses-1 {
			fmt.Println("This ID is not valid")
		} else {
			proc.serversIDs = append(proc.serversIDs, proc.ourID)
			break
		}
	}

	proc.server = listeningSockStrings[proc.ourID].server    //server == true//Client == false

	for i := 0; i < proc.nrOfProcesses; i++ {
		proc.nodeIDs = append(proc.nodeIDs, i)
	}

	wg.Add(2)
	// listening goroutine
	go func() {
		listener, listenErr := net.Listen("tcp", listeningSockStrings[proc.ourID].address)
		fmt.Println("listen message: ", listener, listenErr)
		tempConnsArray := make([]net.Conn, proc.nrOfProcesses-1)
		counter := 0
		counterServer := 1
		// we need this statement otherwise the client will be stuck (since he will receive only 3 connections)
		if !proc.server {
			counter++
			counterServer = 3
		}

		for {
			fmt.Println("Before accept ", counter)
			tempConnsArray[counter], acceptErr = listener.Accept()
			//After the connection is mmade, the listening process waits for the remote process to declare its id

			//remoteProcID_str, _ := bufio.NewReader(tempConnsArray[counter]).ReadString('\n') //Waiting for the id of the connected process
			incomingMess := IncomingInitialMessage{}
			err2 := json.NewDecoder(tempConnsArray[counter]).Decode(&incomingMess)
			if err2 != nil {
				fmt.Println("The initial message was not received! ", err2.Error())
			}

			remoteProcID := incomingMess.ProcId
			if incomingMess.Server { // if we receive a server (if we are client or server)
				proc.listeningConns[remoteProcID] = tempConnsArray[counter]
				fmt.Println("Yoooo received the server id: ", remoteProcID)
				proc.serversIDs = append(proc.serversIDs, remoteProcID)
				counterServer++
			} else if proc.server { // if we are a server and receive a client
				proc.listeningConnsClients[remoteProcID] = tempConnsArray[counter]
				fmt.Println("Yoooo received the client id: ", remoteProcID)
			}
			counter++

			if counter == proc.nrOfProcesses-1 {
				proc.nrOfConnectedServers = counterServer
				break
			}
		}
		fmt.Println("Broke the loop")
		wg.Done()
	}()

	go func() { //Dialing goroutine
		for i, socket := range listeningSockStrings {
			connectionStr := socket.address
			if i != proc.ourID {
				for {
					var conn net.Conn

					if proc.server { // we dial only if we are the server (to everybody)
						if socket.server { // dial a server
							proc.dialingConns[i], err = net.Dial("tcp", connectionStr)
							conn = proc.dialingConns[i]
						} else if !socket.server { // dial a client
							proc.dialingConnsClients[i], err = net.Dial("tcp", connectionStr)
							conn = proc.dialingConnsClients[i]
						}
					} else if !proc.server { // or if we are the client
						if socket.server { // but the socket is a server
							proc.dialingConns[i], err = net.Dial("tcp", connectionStr)
							conn = proc.dialingConns[i]
						} else if !socket.server { // if we are a client, we don't want to connect to the other client(s)
							break
						}
					}
					if err == nil && conn != nil {
						// After having successfully dialed, we should send our id to the listening process
						initialMsg := IncomingInitialMessage{ProcId: proc.ourID, Server: proc.server}
						err2 := json.NewEncoder(conn).Encode(initialMsg)
						if err2 != nil {
							fmt.Println("Error in the dialing connection ", err2.Error())
						}
						fmt.Println("dialed ", i, " and sent my id ", proc.ourID)

						break
					} else if err != nil {
						fmt.Println(err.Error())
					}

					time.Sleep(time.Second * 2)
				}
				fmt.Println("yoooo dialed")
			}
		}
		wg.Done()
	}()

	return &proc
}
// goroutine that wait for incoming messages and differentiate them between:
// Heartbeat, Prepare(paxos), Promise(paxos), Accept(Paxos), Learn(Paxos)
// creates a number of goroutine equal to the number of servers
func (proc Process) readMessages() {
	for index, conn := range proc.listeningConns {
		wg.Add(1)
		go func(index int, conn net.Conn) {
			for {
				var data Data
				err := json.NewDecoder(conn).Decode(&data)
				if err != nil {
					if socketIsClosed(err) {
						proc.closeTheConnection(index, conn, false)
						break
					}
				}

				if data.TypeOfMsg != "" {

					if data.TypeOfMsg != "heartbeat" {
						fmt.Println("RECEIVING FROM SERVER", index, ":", data.TypeOfMsg)
					}

					switch data.TypeOfMsg {
					case "prepare":
						var prp singlepaxos.Prepare
						// unmarshal (decode) the prepare
						err := json.Unmarshal(data.Data, &prp)
						if err != nil {
							fmt.Println("error in", data.TypeOfMsg, ":", err)
						}
						fmt.Println("->RECEIVED:", prp)
						proc.acceptor.DeliverPrepare(prp)
					case "promise":
						var prm singlepaxos.Promise
						// unmarshal (decode) the promise
						err := json.Unmarshal(data.Data, &prm)
						if err != nil {
							fmt.Println("error in", data.TypeOfMsg, ":", err)
						}
						fmt.Println("->RECEIVED:", prm)
						proc.proposer.DeliverPromise(prm)
					case "accept":
						var acc singlepaxos.Accept
						// unmarshal (decode) the accept
						err := json.Unmarshal(data.Data, &acc)
						if err != nil {
							fmt.Println("error in", data.TypeOfMsg, ":", err)
						}
						fmt.Println("->RECEIVED:", acc)
						proc.acceptor.DeliverAccept(acc)
					case "learn":
						var lrn singlepaxos.Learn
						// unmarshal (decode) the learn
						err := json.Unmarshal(data.Data, &lrn)
						if err != nil {
							fmt.Println("error in", data.TypeOfMsg, ":", err)
						}
						fmt.Println("->RECEIVED:", lrn)
						proc.learner.DeliverLearn(lrn)
					case "heartbeat":
						var hb detector.Heartbeat
						// unmarshal (decode) the heartbeat
						err := json.Unmarshal(data.Data, &hb)
						if err != nil {
							fmt.Println("error in", data.TypeOfMsg, ":", err)
						}
						if hb.Request {
							// We received a REQUEST, send an Heartbeat (request)
							proc.failureDetector.DeliverHeartbeat(hb)
						} else {
							// We received a RESPONSE, send Heartbeat (response)
							proc.failureDetector.DeliverHeartbeat(hb)

							sendHeartbeat := make([]detector.Heartbeat, 0)
							sendHeartbeat = append(sendHeartbeat, hb)
							// and send the real RESPONSE to remoteProcID
							proc.sendHeartbeats(sendHeartbeat)
						}
					}
					time.Sleep(500 * time.Millisecond) // wait to do something else
				}
			}
			wg.Done()
		}(index, conn)
	}
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

// function to send each heartbeats to the connecting processes
func (proc Process) sendHeartbeats(heartbeats []detector.Heartbeat) {
	for index, hb := range heartbeats {
		// send the Heartbeat only if the receiver is different from ourself
		// and the sender is ourself
		if hb.To != proc.ourID && hb.From == proc.ourID {
			if conn, ok := proc.dialingConns[hb.To]; ok {
				marshHB, _ := json.Marshal(hb)
				data := Data{TypeOfMsg: "heartbeat", Data: marshHB}
				err := json.NewEncoder(conn).Encode(data)
				if err != nil {
					if socketIsClosed(err) {
						proc.closeTheConnection(index, conn, false)
						break
					}
				}
				// fmt.Println("SEND TO SERVER:", hb.To, data.TypeOfMsg)
				time.Sleep(500 * time.Millisecond) // wait to do send something else
			}
		}
	}
}

// send the paxos data according to the type
func (proc Process) sendPaxosData(data Data, to int) {
	if conn, ok := proc.dialingConns[to]; ok { // if the connection exist
		err := json.NewEncoder(conn).Encode(data)
		if err != nil {
			if socketIsClosed(err) {
				proc.closeTheConnection(to, conn, false)
				return
			}
		}
		fmt.Println("SEND TO SERVER", to, ":", data.TypeOfMsg)
		// time.Sleep(500 * time.Millisecond) // wait to do send something else
	}
}

// wait for receiving (locally) an Heartbeat RESPONSE, in order to respond with a real Heartbeat RESPONSE
func (proc Process) waitForHBSend(hbSend chan detector.Heartbeat) {
	for {
		select {
		case hbResp := <-hbSend:
			// send the Heartbeat (RESPONSE) only to ourself
			if hbResp.To == proc.ourID {
				proc.failureDetector.DeliverHeartbeat(hbResp)
			}

			sendHeartbeat := make([]detector.Heartbeat, 0)
			sendHeartbeat = append(sendHeartbeat, hbResp)
			proc.sendHeartbeats(sendHeartbeat) // and sent
		default:
			time.Sleep(300 * time.Millisecond)
		}
	}
}

// wait for some write in the subscriber channel in order to print out the new leader
func (proc Process) updateSubscribe() {
	for {
		for n := 0; n < len(proc.subscriber); n++ {
			time.Sleep(time.Millisecond * 500)
			select {
			case gotLeader := <-proc.subscriber[n]:
				// Got publication, change the leader
				fmt.Println("----------- NEW LEADER: ", gotLeader, "-----------")
			}
		}
	}
}



func main() {
	delta := time.Second * 3

	proc := load_configurations()
	wg.Wait()
	fmt.Println("CONNECTIONS ESTABLISHED")

	if proc.server {
		proc.handleServer(delta)
	} else {
		proc.handleClient()
	}

}


func socketIsClosed(err error) bool {
	if strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "forcibly closed by the remote host") {
		return true
	}
	return false
}
func (proc Process) closeTheConnection(index int, conn net.Conn, client bool) {
	conn.Close()
	var typo string
	if !client {
		delete(proc.listeningConns, index)
		delete(proc.dialingConns, index)
		typo = "server"
	} else {
		delete(proc.listeningConnsClients, index)
		delete(proc.dialingConnsClients, index)
		typo = "client"
	}

	fmt.Println("********** Connection closed with ", typo, " ", index)
}
func terminal_input(message string) string {
	var input string
	fmt.Print(message, ": ")
	fmt.Scanln(&input)
	return input
}
func get_int_from_terminal(message string) int {
	s := terminal_input(message)
	x, _ := strconv.ParseInt(s, 10, 32)
	return int(x)
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
/////////////////////////////////SERVER//////////////SERVER///////////////////////////SERVER//////////////////////////////////////

// MAIN FUNCTION FOR THE SERVER
func (proc Process) handleServer(delta time.Duration) {
	// add the leader Detector
	proc.leaderDetector = detector.NewMonLeaderDetector(proc.serversIDs)
	fmt.Println("---------- ACTUAL LEADER: process n. ", proc.leaderDetector.Leader())
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

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
/////////////////////////////////CLIENT//////////////CLIENT///////////////////////////CLIENT//////////////////////////////////////

func (proc Process) handleClient() {
	wg.Add(2)
	go sendValue(proc)
	go waitForLearn(proc)
	wg.Wait()

}

func sendValue(proc Process) {
	for {
		var value singlepaxos.Value
		strValue := terminal_input("Insert a value: ")
		value = singlepaxos.Value(strValue)

		for index := range proc.dialingConns { // send the value to each server
			if conn, ok := proc.dialingConns[index]; ok {
				err := json.NewEncoder(conn).Encode(value)
				if err != nil {
					if socketIsClosed(err) {
						proc.closeTheConnection(index, conn, false)
						return
					}
				}
				fmt.Println("SEND VALUE ", value, " TO SERVER: ", index)
				time.Sleep(300 * time.Millisecond) // wait to do something else
			}
		}
		time.Sleep(10 * time.Second) // wait to do something else
	}
}

func waitForLearn(proc Process) {
	for index, conn := range proc.listeningConns {
		wg.Add(1)
		go func(index int, conn net.Conn) {
			for {
				var message singlepaxos.Value
				err := json.NewDecoder(conn).Decode(&message)
				if err != nil {
					if socketIsClosed(err) {
						proc.closeTheConnection(index, conn, false)
						break
					}
				}

				fmt.Println("RECEIVING LEARNED VALUE FROM SERVER ", index, ": ", message)
			}
			wg.Done()
		}(index, conn)
	}
}
