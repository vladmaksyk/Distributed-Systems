package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab1/detector"
	"github.com/uis-dat520-s18/glabs/grouplab2/singlepaxos"
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

func load_configurations() *Process {
	proc := Process{nrOfConnectedServers: 0}

	var err error
	var acceptErr error
	proc.nrOfProcesses = get_int_from_terminal("Specify the total number of processes: ")
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

	proc.server = listeningSockStrings[proc.ourID].server

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

// socketIsClosed is a helper method to check if a listening socket has been closed.
func socketIsClosed(err error) bool {
	if strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "forcibly closed by the remote host") {
		return true
	}
	return false
}

// Close the connection with a process if there is an error
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
