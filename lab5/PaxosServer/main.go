package main

import (
	"net"
	"sync"
	"time"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"math"
	"math/rand"

	colorable "github.com/mattn/go-colorable"
	log "github.com/sirupsen/logrus"
	"github.com/uis-dat520-s18/glabs/grouplab1/detector"
	"github.com/uis-dat520-s18/glabs/grouplab3/bank"
	"github.com/uis-dat520-s18/glabs/grouplab3/multipaxos"
	"github.com/montanaflynn/stats"

)

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
	receivedValue         chan bool                    // only for client
	failureDetector       *detector.EvtFailureDetector // only server
	leaderDetector        *detector.MonLeaderDetector  // only server
	subscriber            []<-chan int                 // only server
	server                bool
	proposer              *multipaxos.Proposer             // only server
	acceptor              *multipaxos.Acceptor             // only server
	learner               *multipaxos.Learner              // only server
	bufferedValues        map[int]*multipaxos.DecidedValue // only server
	accountsList          map[int]*bank.Account            // only server
	valueSentToClients    map[int]bool                     // value to store a boolean if we already sent the value to the client
}

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
}

var listeningSockStrings = []socket{
	socket{address: "localhost:12100", server: true},
	socket{address: "localhost:12101", server: true},
	socket{address: "localhost:12102", server: true},
	socket{address: "localhost:12103", server: false},
	socket{address: "localhost:12104", server: false},
	socket{address: "localhost:12105", server: false},
}

var wg = sync.WaitGroup{}
var suspectChangeChan chan int
var restoreChangeChan chan int

var (
	mainServer       int
	clientID         string
	automaticMode    bool
	nrOperations     int
	start            time.Time
	end              time.Time
	timeOfOperations []float64
	lastSavedValue multipaxos.Value
	value          multipaxos.Value
	maxSequence   int
	indexOfServer int
	opValue       int
	amountValue   int
	accountNumber int
	timeoutSignal *time.Ticker
	stop          chan struct{}
)


func load_configurations() (*Process, string) {
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
			log.Error("This ID is not valid")
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
		log.WithFields(log.Fields{"listen": listener, "error": listenErr}).Debug("listen message: ")
		tempConnsArray := make([]net.Conn, proc.nrOfProcesses-1)
		counter := 0
		counterServer := 1
		// we need this statement otherwise the client will be stuck (since he will receive only 3 connections)
		if !proc.server {
			for _, sock := range listeningSockStrings {
				if !sock.server {
					counter++
				}
			}
			counter-- // number of client - 1 !!!!!!!!
			counterServer = 3
		}

		for {
			log.WithField("counter", counter).Debug("Before accept ")
			tempConnsArray[counter], acceptErr = listener.Accept()
			//After the connection is mmade, the listening process waits for the remote process to declare its id

			incomingMess := IncomingInitialMessage{}
			err2 := json.NewDecoder(tempConnsArray[counter]).Decode(&incomingMess)
			if err2 != nil {
				log.WithField("error", err2.Error()).Error("The initial message was not received! ")
			}

			remoteProcID := incomingMess.ProcId
			if incomingMess.Server { // if we receive a server (if we are client or server)
				proc.listeningConns[remoteProcID] = tempConnsArray[counter]
				log.WithField("id", remoteProcID).Debug("Yoooo received the server id: ")
				proc.serversIDs = append(proc.serversIDs, remoteProcID)
				counterServer++
			} else if proc.server { // if we are a server and receive a client
				proc.listeningConnsClients[remoteProcID] = tempConnsArray[counter]
				log.WithField("id", remoteProcID).Debug("Yoooo received the client id: ")
			}
			counter++

			if counter == proc.nrOfProcesses-1 {
				proc.nrOfConnectedServers = counterServer
				log.WithField("id", proc.listeningConns).Warn("List Conn: ")
				log.WithField("id", proc.listeningConnsClients).Warn("List Conn client: ")
				break
			}
		}
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
							log.WithField("error", err2.Error()).Error("Error in the dialing connection ")
						}
						log.WithField("id", proc.ourID).Debug("dialed ", i, " and sent my id ")

						break
					} else if err != nil {
						log.Error(err.Error())
					}

					time.Sleep(time.Second * 2)
				}
			}
		}
		wg.Done()
	}()

	return &proc, listeningSockStrings[proc.ourID].address
}

func (proc Process) handleServer(delta time.Duration) {
	// add the leader Detector
	proc.leaderDetector = detector.NewMonLeaderDetector(proc.serversIDs)
	log.Debug("---------- ACTUAL LEADER: process n. ", proc.leaderDetector.GetLeader())
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

	prepareOut := make(chan multipaxos.Prepare, 8)
	acceptOut := make(chan multipaxos.Accept, 8)
	promiseOut := make(chan multipaxos.Promise, 8)
	learnOut := make(chan multipaxos.Learn, 8)
	decidedOut := make(chan multipaxos.DecidedValue, 8)
	proc.proposer = multipaxos.NewProposer(proc.ourID, proc.nrOfConnectedServers, -1, proc.leaderDetector, prepareOut, acceptOut)
	proc.proposer.Start()
	proc.acceptor = multipaxos.NewAcceptor(proc.ourID, promiseOut, learnOut)
	proc.acceptor.Start()
	proc.learner = multipaxos.NewLearner(proc.ourID, proc.nrOfConnectedServers, decidedOut)
	proc.learner.Start()

	go proc.startPaxosMethod(prepareOut, acceptOut, promiseOut, learnOut, decidedOut)

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

func (proc Process) startPaxosMethod(prepareOut chan multipaxos.Prepare,acceptOut chan multipaxos.Accept, promiseOut chan multipaxos.Promise,learnOut chan multipaxos.Learn, decidedOut chan multipaxos.DecidedValue) {
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
			// receive a promise; deliver the promise to prm.To
			// marshal (encode) the promise
			marshPrm, _ := json.Marshal(prm)
			msg := Data{TypeOfMsg: "promise", Data: marshPrm}
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
		case decidedVal := <-decidedOut:
			// handle the decided value (all the servers, not only the leader)
			proc.handleDecideValue(decidedVal)
		}
	}
}

func (proc Process) sendPaxosData(data Data, to int) {
	if conn, ok := proc.dialingConns[to]; ok { // if the connection exist
		err := json.NewEncoder(conn).Encode(data)
		if err != nil {
			if socketIsClosed(err) {
				proc.closeTheConnection(to, conn, false)
				return
			}
		}
		log.Info("SEND TO SERVER ", to, ": ", data.TypeOfMsg)
	}
}
// handle the transactions for every server,
// but only the leader will send the reponse to the client
func (proc Process) handleDecideValue(decidedVal multipaxos.DecidedValue) {
	adu := proc.proposer.GetADU()
	if decidedVal.SlotID > adu+1 {
		proc.bufferedValues[int(adu)+1] = &decidedVal
		return
	}
	if !decidedVal.Value.Noop {
		if proc.accountsList[decidedVal.Value.AccountNum] == nil {
			// create and store new account with a balance of zero
			proc.accountsList[decidedVal.Value.AccountNum] = &bank.Account{Number: decidedVal.Value.AccountNum, Balance: 0}
		}
		// apply transaction from value to account
		transactionRes := proc.accountsList[decidedVal.Value.AccountNum].Process(decidedVal.Value.Txn)

		if proc.ourID == proc.leaderDetector.Leader() {
			// create response with appropriate transaction result, client id and client seq
			response := multipaxos.Response{ClientID: decidedVal.Value.ClientID, ClientSeq: decidedVal.Value.ClientSeq, TxnRes: transactionRes}
			marshHB, _ := json.Marshal(response)
			msg := Data{TypeOfMsg: "response", Data: marshHB}
			clientID, _ := strconv.Atoi(decidedVal.Value.ClientID)
			// forward response to client handling module
			proc.sendValueOrRedirectToCliente(clientID, msg)
		}
	}
	// increment adu by 1 (increment decided slot for proposer)
	proc.proposer.IncrementAllDecidedUpTo()
	if proc.bufferedValues[int(adu)+1] != nil {
		proc.handleDecideValue(*proc.bufferedValues[int(adu)+1])
	}
}

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
						log.WithFields(log.Fields{"id": index, "type": data.TypeOfMsg}).Info("RECEIVING FROM SERVER: ")
					}

					switch data.TypeOfMsg {
					case "prepare":
						var prp multipaxos.Prepare
						// unmarshal (decode) the prepare
						err := json.Unmarshal(data.Data, &prp)
						if err != nil {
							log.WithFields(log.Fields{"type": data.TypeOfMsg, "err": err}).Error("error")
						}
						log.WithField("value", prp).Debug("->RECEIVED: ")
						proc.acceptor.DeliverPrepare(prp)
					case "promise":
						var prm multipaxos.Promise
						// unmarshal (decode) the promise
						err := json.Unmarshal(data.Data, &prm)
						if err != nil {
							log.WithFields(log.Fields{"type": data.TypeOfMsg, "err": err}).Error("error")
						}
						log.WithField("value", prm).Debug("->RECEIVED: ")
						proc.proposer.DeliverPromise(prm)
					case "accept":
						var acc multipaxos.Accept
						// unmarshal (decode) the accept
						err := json.Unmarshal(data.Data, &acc)
						if err != nil {
							log.WithFields(log.Fields{"type": data.TypeOfMsg, "err": err}).Error("error")
						}
						log.WithField("value", acc).Debug("->RECEIVED: ")
						proc.acceptor.DeliverAccept(acc)
					case "learn":
						var lrn multipaxos.Learn
						// unmarshal (decode) the learn
						err := json.Unmarshal(data.Data, &lrn)
						if err != nil {
							log.WithFields(log.Fields{"type": data.TypeOfMsg, "err": err}).Error("error")
						}
						log.WithField("value", lrn).Debug("->RECEIVED: ")
						proc.learner.DeliverLearn(lrn)
					case "heartbeat":
						var hb detector.Heartbeat
						// unmarshal (decode) the heartbeat
						err := json.Unmarshal(data.Data, &hb)
						if err != nil {
							log.WithFields(log.Fields{"type": data.TypeOfMsg, "err": err}).Error("error")
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
				}
			}
			wg.Done()
		}(index, conn)
	}
}

func (proc Process) waitForClientValueAndDeliverPrepare() {
	for index, conn := range proc.listeningConnsClients {
		wg.Add(1)
		go func(index int, conn net.Conn) {
			for {
				var message multipaxos.Value
				err := json.NewDecoder(conn).Decode(&message)
				if err != nil {
					if socketIsClosed(err) {
						proc.closeTheConnection(index, conn, true)
						break
					}
				}

				log.WithFields(log.Fields{"id": index, "msg": message}).Info("RECEIVING VALUE FROM CLIENT:")

				// if the process is the leader, start the multipaxos method
				if proc.ourID == proc.leaderDetector.GetLeader() {
					proc.proposer.DeliverClientValue(message)
				} else {
					// return a redirect to the client
					leaderIndex := proc.leaderDetector.GetLeader()
					marshHB, _ := json.Marshal(leaderIndex)
					msg := Data{TypeOfMsg: "redirect", Data: marshHB}
					// forward response to client handling module
					proc.sendValueOrRedirectToCliente(index, msg)
				}
			}
			wg.Done()
		}(index, conn)
	}}

func (proc Process) sendValueOrRedirectToCliente(clientID int, msg Data) {
	if conn, ok := proc.dialingConnsClients[clientID]; ok {
		err := json.NewEncoder(conn).Encode(msg)
		if err != nil {
			if socketIsClosed(err) {
				proc.closeTheConnection(clientID, conn, false)
				return
			}
		}
		log.WithFields(log.Fields{"id": clientID, "msg": msg.TypeOfMsg}).Info("SEND VALUE TO CLIENT: ")
	}
}

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
			}
		}
	}
}

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
		}
	}
}

func (proc Process) updateSubscribe() {
	for {
		for n := 0; n < len(proc.subscriber); n++ {
			select {
			case gotLeader := <-proc.subscriber[n]:
				// Got publication, change the leader
				log.Info("----------- NEW LEADER: ", gotLeader, " -----------")
			}
		}
	}
}
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// MAIN FUNCTION FOR THE SERVER

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func main() {
	delta := time.Second * 2
	// Only log the warning severity or above.
	log.SetLevel(log.DebugLevel) // JUST FOR TEST
	log.SetOutput(colorable.NewColorableStdout())

	proc, address := load_configurations()
	wg.Wait()
	log.Info("CONNECTIONS ESTABLISHED")


	if proc.server {
		proc.bufferedValues = make(map[int]*multipaxos.DecidedValue)
		proc.accountsList = make(map[int]*bank.Account)
		proc.handleServer()
	} else {
		proc.handleClient()
	}
	wg.Wait()
}

///////////////////////////////////////CLIENT/////////////////////////////////////////////////////////////////////////////////////////
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// MAIN PART OF THE CLIENT
func (proc Process) handleClient() {
	mainServer = -1 // server to connect (leader)
	nrOperations = 0
	maxSequence = 0
	clientID = strconv.Itoa(proc.ourID)
	proc.receivedValue = make(chan bool, 8)
	stop = make(chan struct{})
	timeOfOperations = make([]float64, 0)

	// choose between manual and automatic mode
	stringMode := terminal_input("Manual mode (m) or Automatic mode (a)? ")
	timeoutSignal = time.NewTicker(time.Second * 10)
	StartSelect(proc) // start the select for loop

	if stringMode == "m" {
		automaticMode = false
	} else if stringMode == "a" {
		automaticMode = true
	}

	proc.DeliverResponse() // a trick to start sending value to the servers
	waitForLearn(proc)     // wait to receive messages

}

// send the value in a manual or automatic mode
func sendValue(proc Process) {
	if automaticMode == false {
		opValue = get_int_from_terminal("Insert an operation (0: Balance, 1: Deposit, 2: Withdrawal): ")
		accountNumber = get_int_from_terminal("Insert an account number: ")
		amountValue = 0
		if opValue > 0 {
			amountValue = get_int_from_terminal("Insert an amount: ")
		}
	} else {
		opValue = rand.Intn(2)
		accountNumber = rand.Intn(1000)
		amountValue = 0
		if opValue > 0 {
			amountValue = rand.Intn(10000)
		}
	}

	transaction := bank.Transaction{Op: bank.Operation(opValue), Amount: amountValue}
	value = multipaxos.Value{ClientID: clientID, ClientSeq: maxSequence, AccountNum: accountNumber, Txn: transaction}

	if mainServer == -1 { // try to connect to a random server (if we didn't receive any redirect)
		indexOfServer = rand.Intn(len(proc.dialingConns) - 1)
	} else { // or to the server indicated by the redirect
		indexOfServer = mainServer
	}
	if conn, ok := proc.dialingConns[indexOfServer]; ok {
		err := json.NewEncoder(conn).Encode(value)
		if err != nil {
			if socketIsClosed(err) {
				proc.closeTheConnection(indexOfServer, conn, false)
				return
			}
		}
		maxSequence++

		lastSavedValue = value
		log.WithFields(log.Fields{"id": indexOfServer, "msg": value}).Info("--> SEND VALUE TO SERVER: ")
	}
}

func StartSelect(proc Process) {   // New
	go func() {
		for {
			select {
			case response := <-proc.receivedValue:
				start = time.Now()
				if response {
					nrOperations++
					if nrOperations < 500 {
						sendValue(proc)
					}
				} else {
					// we received a redirect
					if conn, ok := proc.dialingConns[mainServer]; ok {
						err := json.NewEncoder(conn).Encode(lastSavedValue)
						if err != nil {
							if socketIsClosed(err) {
								proc.closeTheConnection(mainServer, conn, false)
								return
							}
						}
						log.WithFields(log.Fields{"id": mainServer, "msg": lastSavedValue}).Info("--> SEND VALUE TO SERVER: ")
					}
				}
			case <-timeoutSignal.C:
				log.Error("TIMEOUT triggered...")
				mainServer = -1 // reset the leader
				if nrOperations < 500 {
					sendValue(proc)
					timeoutSignal = time.NewTicker(time.Second * 10)
				} else {
					Stop()
				}
			case <-stop:
				// create the statistics
				mean, _ := stats.Mean(timeOfOperations)
				log.WithFields(log.Fields{"mean": mean}).Info("Mean in seconds:")
				minimum, _ := stats.Min(timeOfOperations)
				log.WithFields(log.Fields{"min": minimum}).Info("Minimum in seconds:")
				maximum, _ := stats.Max(timeOfOperations)
				log.WithFields(log.Fields{"max": maximum}).Info("Maximum in seconds:")
				median, _ := stats.Median(timeOfOperations)
				log.WithFields(log.Fields{"median": median}).Info("Median in seconds:")
				variance, _ := stats.Variance(timeOfOperations)
				stddev := math.Sqrt(variance)
				log.WithFields(log.Fields{"std": stddev}).Info("Standard deviation in seconds:")
				percentile, _ := stats.Percentile(timeOfOperations, 99)
				log.WithFields(log.Fields{"99perc": percentile}).Info("99 Percentile in seconds:")
				return
			}
		}
	}()
}



func waitForLearn(proc Process) {
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
					switch data.TypeOfMsg {
					case "response":
						var response multipaxos.Response
						// unmarshal (decode) the response
						err := json.Unmarshal(data.Data, &response)
						if err != nil {
							log.WithFields(log.Fields{"type": data.TypeOfMsg, "err": err}).Error("error")
						}
						log.WithFields(log.Fields{"id": index, "msg": response}).Info("<-- RECEIVING RESPONSE FROM SERVER : ")
						log.WithField("nr:", nrOperations).Warn("Number of operations:")
						if nrOperations < 500 {
							end = time.Now()
							timeOfOperations = append(timeOfOperations, end.Sub(start).Seconds())
							time.Sleep(time.Millisecond * 50)
							proc.DeliverResponse()
						} else {
							Stop()
						}
					case "redirect":
						var leader int
						// unmarshal (decode) the leader
						err := json.Unmarshal(data.Data, &leader)
						if err != nil {
							log.WithFields(log.Fields{"type": data.TypeOfMsg, "err": err}).Error("error")
						}
						log.WithFields(log.Fields{"id": index, "to": leader}).Info("<-- RECEIVING REDIRECT FROM SERVER : ")

						mainServer = leader
						proc.DeliverRedirect()
					}
				}
			}
			wg.Done()
		}(index, conn)
	}
}


//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func Stop() {
	stop <- struct{}{} // stop the automatic send
}
func (proc Process) DeliverResponse() {
	proc.receivedValue <- true
}
func (proc Process) DeliverRedirect() {
	proc.receivedValue <- false
}
func (proc Process) resetMapValueSentToClient() {
	for i := range proc.dialingConnsClients {
		proc.valueSentToClients[i] = false
	}
}
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
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

	log.Error("********** Connection closed with ", typo, " ", index)
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
