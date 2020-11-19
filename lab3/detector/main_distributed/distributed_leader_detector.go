package main

import (
	"fmt"
	"net"
	"sync"
	"time"
	"bufio"
	"encoding/gob"
	"strconv"
	"strings"
	"github.com/uis-dat520-s2019/GMaksyk_18/grouplab1/detector"
)

var listeningSockStrings = []string{"localhost:1200", "localhost:1201", "localhost:1202"}

type Process struct {
	ourID                  int
	nrOfProcesses          int
	nodeIDs                []int
	listeningConns         map[int]net.Conn
	dialingConns           map[int]net.Conn
	nrOfConnectedProcesses int
	failureDetector        *detector.EvtFailureDetector
	leaderDetector         *detector.MonLeaderDetector
	subscriber             []<-chan int
}

var wg = sync.WaitGroup{}
var suspectChangeChan chan int
var restoreChangeChan chan int


func establishConnections() *Process {
	proc := Process{nrOfConnectedProcesses: 0}

	var err error
	var acceptErr error
	proc.nrOfProcesses = get_int_from_terminal("Specify the total number of processes: ")
	proc.dialingConns = make(map[int]net.Conn)
	proc.listeningConns = make(map[int]net.Conn)

	for {
		proc.ourID = get_int_from_terminal("Specify the ID of the process")
		if proc.ourID < 0 || proc.ourID > proc.nrOfProcesses-1 {
			fmt.Println("This ID is not valid")
		} else {
			break
		}
	}

	for i := 0; i < proc.nrOfProcesses; i++ {
		proc.nodeIDs = append(proc.nodeIDs, i)
	}


	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	wg.Add(2)
	// listening goroutine
	go func() {
		fmt.Println("Listening...")
		listener, listenErr := net.Listen("tcp", listeningSockStrings[proc.ourID])
		fmt.Println("Listen message: ", listener, listenErr)
		tempConnsArray := make([]net.Conn, proc.nrOfProcesses-1)
		counter := 0

		for {

			tempConnsArray[counter], acceptErr = listener.Accept()

			//After the connection is mmade, the listening process waits for the remote process to declare its id
			remoteProccId_str, _ := bufio.NewReader(tempConnsArray[counter]).ReadString('\n') //Waiting for the id of the connected process

			remoteProccId_str = strings.TrimRight(remoteProccId_str, "\n")
			remoteProccId, convErr := strconv.Atoi(remoteProccId_str)
			fmt.Println("Conversion error ", convErr)

			proc.listeningConns[remoteProccId] = tempConnsArray[counter]
			fmt.Println("Received the id: int->", remoteProccId, "->", remoteProccId_str, proc.listeningConns[remoteProccId])
			counter++

			if counter == proc.nrOfProcesses-1 {
				proc.nrOfConnectedProcesses = counter
				break
			}
			fmt.Println("After accept")
		}
		fmt.Println("Broke the loop")
		wg.Done()
	}()
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	go func() { //Dialing goroutine
		for i, connectionStr := range listeningSockStrings {
			if i != proc.ourID {

				for {
					proc.dialingConns[i], err = net.Dial("tcp", connectionStr)
					if err == nil {
						//After having successfully dialed, we should send our id to the listening process
						fmt.Fprintf(proc.dialingConns[i], "%d\n", proc.ourID)
						fmt.Println("Dialed ", i, proc.dialingConns[i], " and sent my id ", proc.ourID)

						break
					}
					fmt.Println("Dialing error: ", err.Error())
					time.Sleep(time.Second * 2)
				}
				fmt.Println("Successfully dialed")
			}
		}
		wg.Done()
	}()

	return &proc
}

// goroutine that wait for incoming messages and choose if the message is a REQUEST or a RESPONSE
func (proc Process) readMessages() {
	for index, conn := range proc.listeningConns {
		wg.Add(1)
		go func(index int, conn net.Conn) {
			for {
				var message detector.Heartbeat
				err := gob.NewDecoder(conn).Decode(&message)
				if err != nil {
					if socketIsClosed(err) {
						proc.closeTheConnection(index, conn)
						break
					}
				}

				fmt.Println("READ: ", message)

				if message.Request {
					// We received a REQUEST, send an Heartbeat (request)
					proc.failureDetector.DeliverHeartbeat(message)
				} else {
					// We received a RESPONSE, send Heartbeat (response)
					proc.failureDetector.DeliverHeartbeat(message)

					sendHeartbeat := make([]detector.Heartbeat, 0)
					sendHeartbeat = append(sendHeartbeat, message)
					// and send the real RESPONSE to remoteProccId
					proc.sendHeartbeats(sendHeartbeat)
				}
				time.Sleep(300 * time.Millisecond) // wait to do something else
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
				err := gob.NewEncoder(conn).Encode(hb)
				if err != nil {
					if socketIsClosed(err) {
						proc.closeTheConnection(index, conn)
						break
					}
				}
				fmt.Println("SEND: ", hb)
				time.Sleep(1000 * time.Millisecond) // wait to do something else
			}
		}
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
			time.Sleep(500 * time.Millisecond)
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
	delta := time.Second * 5
	proc := establishConnections()
	wg.Wait()
	fmt.Println("process struct: ", proc.ourID, proc.nrOfProcesses, proc.nodeIDs, delta)
	fmt.Println("CONNECTIONS ESTABLISHED")

	// add the leader Detector
	proc.leaderDetector = detector.NewMonLeaderDetector(proc.nodeIDs)
	fmt.Println("---------- ACTUAL LEADER: process n. ", proc.leaderDetector.Leader())

	proc.subscriber = make([]<-chan int, 1) // subscribe for leader changes
	for n := 0; n < len(proc.subscriber); n++ {
		proc.subscriber[n] = proc.leaderDetector.Subscribe() // our subscriber
	}

	// add the eventual failure detector

	hbOut := make(chan detector.Heartbeat, 8)
	fd := detector.NewEvtFailureDetector(proc.ourID, proc.nodeIDs, proc.leaderDetector, delta, hbOut)
	proc.failureDetector = fd
	proc.failureDetector.Start()

	// ----- split here

	wg.Add(2)
	proc.readMessages() // start the goroutine to read messages and decode them

	// create a slice of heartbeats to send to the other connected processes
	sliceHeartbeats := make([]detector.Heartbeat, 0)
	for _, id := range proc.nodeIDs {
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

//////////////////////////////////////////////////////////////////////////////
// socketIsClosed is a helper method to check if a listening socket has been closed.
func socketIsClosed(err error) bool {
	if strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "forcibly closed by the remote host") {
		return true
	}
	return false
}

// Close the connection with a process if there is an error
func (proc Process) closeTheConnection(index int, conn net.Conn) {
	conn.Close()
	delete(proc.listeningConns, index)
	delete(proc.dialingConns, index)
	fmt.Println("********** Connection closed with process ", index)
}

// function to get the input from the terminal
func terminal_input(message string) string {
	var input string
	fmt.Print(message, ": ")
	fmt.Scanln(&input)
	return input
}

// function to get the input from the terminal
func get_int_from_terminal(message string) int {
	s := terminal_input(message)
	x, _ := strconv.ParseInt(s, 10, 32)
	return int(x)
}
