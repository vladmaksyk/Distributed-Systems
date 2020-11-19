package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"flag"
	"sync"
	"net/http"
	"time"
	log "github.com/sirupsen/logrus"
	"github.com/uis-dat520-s18/glabs/grouplab1/detector"
	"github.com/uis-dat520-s18/glabs/grouplab4/multipaxos"
	"github.com/gorilla/websocket"
	colorable "github.com/mattn/go-colorable"
	"github.com/uis-dat520-s18/glabs/grouplab4/bank"

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
}

type DataFromClient struct {
	From        string
	To          string
	Transaction string
	Value       string
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
	webSocketConns        *websocket.Conn              // connection with the web socket
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
	bridgeChannel         chan multipaxos.Response
}

var (
	wg = sync.WaitGroup{}
	suspectChangeChan chan int
	restoreChangeChan chan int

	addrs = []*string{
		flag.String("client1", "127.0.0.1:8080", "http service address"),
		flag.String("client2", "127.0.0.1:8888", "http service address"),
	}
	listeningSockStrings = []socket{
		socket{address: "localhost:12100", server: true},
		socket{address: "localhost:12101", server: true},
		socket{address: "localhost:12102", server: true},
	}
)

var upgrader = websocket.Upgrader{}
var ClientSeq = 0

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
func (proc Process) readMessages() {
	for index, conn := range proc.listeningConns {
		wg.Add(1)
		go func(index int, conn net.Conn) {
			for {
				var data Data
				err := json.NewDecoder(conn).Decode(&data) //Waits for a message from every listening connection (listeningConns).
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
			proc.proposer.DeliverPromise(prm)
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
			message := proc.handleDecideValue(decidedVal)
			// only if we are the leader, we write the message in the channel
			if proc.ourID == proc.leaderDetector.Leader() {
				proc.bridgeChannel <- message
			}
		}
	}
}
func (proc Process) handleDecideValue(decidedVal multipaxos.DecidedValue) (msg multipaxos.Response) {
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
		var transactionRes bank.TransactionResult

		if decidedVal.Value.Txn.Op == 3 { // if the Operation is a "Transfer"
			firstTransaction := bank.Transaction{
				Op:     2, // Withdrawal the amount of money from the accont selected in the variable From
				From:   decidedVal.Value.Txn.From,
				To:     decidedVal.Value.Txn.To,
				Amount: decidedVal.Value.Txn.Amount,
			}
			transactionRes = proc.accountsList[decidedVal.Value.AccountNum].Process(firstTransaction) // save the first transaction and show this result
			secondTransaction := bank.Transaction{
				Op:     1, // Deposit the money in the other account selected in the variable To
				From:   decidedVal.Value.Txn.From,
				To:     decidedVal.Value.Txn.To,
				Amount: decidedVal.Value.Txn.Amount,
			}

			if proc.accountsList[decidedVal.Value.Txn.To.Number] == nil {
				// create and store new account with a balance of zero
				proc.accountsList[decidedVal.Value.Txn.To.Number] = &bank.Account{Number: decidedVal.Value.Txn.To.Number, Balance: 0}
			}
			proc.accountsList[decidedVal.Value.Txn.To.Number].Process(secondTransaction) // save the second transaction but don't show the result
		} else {
			// for the all other transactions just call a simple process function
			transactionRes = proc.accountsList[decidedVal.Value.AccountNum].Process(decidedVal.Value.Txn)
		}
		// create response with appropriate transaction result, client id and client seq
		response := multipaxos.Response{ClientID: decidedVal.Value.ClientID, ClientSeq: decidedVal.Value.ClientSeq, TxnRes: transactionRes}
		if proc.ourID == proc.leaderDetector.Leader() {
			msg = response
		}
	}
	// increment adu by 1 (increment decided slot for proposer)
	proc.proposer.IncrementAllDecidedUpTo()
	if proc.bufferedValues[int(adu)+1] != nil {
		msg = proc.handleDecideValue(*proc.bufferedValues[int(adu)+1])
	}

	return msg
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

	// variable to read the decided value from the paxos method to the webSocket
	proc.bridgeChannel = make(chan multipaxos.Response, 20)

	wg.Add(4)
	// --- paxos implementation
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
	proc.readMessages() // from the servers. actual messages received in the network layer.

	// create a slice of initial heartbeats to send to the other connected processes
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

	http.HandleFunc("/", proc.serveHome)
	proc.webSocket() // start the web socket; the leader is listening

	wg.Wait()
}


func (proc Process) webSocket() {
	// only the leader communicate with the client(s)
	if proc.ourID == proc.leaderDetector.GetLeader() {
		http.HandleFunc("/ws"+strconv.Itoa(proc.ourID), proc.serveWs)

		for addrInd := 0; addrInd < len(addrs); addrInd++ {
			go func(addrInd int) {
				log.Fatal(http.ListenAndServe(*addrs[addrInd], nil)) // the leader listen to all the address saved in the addrs variable
			}(addrInd)
		}
	}
}

// function that handle the communication between server and client
func (proc Process) serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	proc.webSocketConns = ws

	for {
		var data DataFromClient

		err := ws.ReadJSON(&data) // read JSON data
		fmt.Println("got data from client")
		if err != nil {
			fmt.Println(data)
			fmt.Println("Got ReadJason Eror")
			fmt.Println(err)
			break
		}

		var mainErr error
		go func() {
			msg = proc.handleMessage(data)
			mainErr = ws.WriteJSON(msg) // send the JSON data
		}()

		if mainErr != nil {
			fmt.Println(mainErr)
			break
		}
	}
}

// function that load the html file of the client
func (proc Process) serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "home.html")
}

// MAIN FUNCTION FOR DELIVER THE MESSAGE FROM THE CLIENT TO PAXOS
func (proc Process) handleMessage(data DataFromClient) (response multipaxos.Response) {

	var (
		message multipaxos.Value
		txn     bank.Transaction
	)

	switch data.Transaction {
	case "Balance":
		txn.Op = 0
	case "Deposit":
		txn.Op = 1
	case "Withdrawal":
		txn.Op = 2
	case "Transfer":
		txn.Op = 3
		accountNumTo, _ := strconv.Atoi(data.To)

		if proc.accountsList[accountNumTo] == nil {
			// if there isn't an account to transfer the money, initialize it
			proc.accountsList[accountNumTo] = &bank.Account{Number: accountNumTo, Balance: 0}
		}
		txn.To = *proc.accountsList[accountNumTo]
	}

	txn.Amount, _ = strconv.Atoi(data.Value)
	message.AccountNum, _ = strconv.Atoi(data.From)

	if proc.accountsList[message.AccountNum] == nil {
		proc.accountsList[message.AccountNum] = &bank.Account{Number: message.AccountNum, Balance: 0}
	}
	txn.From = *proc.accountsList[message.AccountNum]

	message.Txn = txn
	message.ClientID = data.From
	message.ClientSeq = ClientSeq

	log.WithFields(log.Fields{"msg": message}).Info("RECEIVING VALUE FROM CLIENT:")
	if proc.ourID == proc.leaderDetector.GetLeader() {
		proc.proposer.DeliverClientValue(message)
	}

	for {
		select {
		case msg := <-proc.bridgeChannel: // wait for the result from Paxos and then increase the sequence
			ClientSeq += 1
			return msg
		}
	}

}

// start the webSocket and handle the listening part for the clients





func main() {
	delta := time.Second * 3
	// Only log the warning severity or above.
	log.SetLevel(log.DebugLevel) // JUST FOR TEST
	log.SetOutput(colorable.NewColorableStdout())

	proc, _ := load_configurations()
	wg.Wait()
	log.Info("CONNECTIONS ESTABLISHED")

	if proc.server {
		proc.bufferedValues = make(map[int]*multipaxos.DecidedValue)
		proc.accountsList = make(map[int]*bank.Account)
		proc.handleServer(delta)
	}

	wg.Wait()
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
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

	log.Error("********** Connection closed with ", typo, " ", index)
	time.Sleep(6 * time.Second) // wait for the subscriber to update the new leader
	proc.webSocket()            // start again the webSocket from a different server
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
