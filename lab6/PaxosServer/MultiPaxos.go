package main

import(
	"fmt"
	"net"
	"sync"
	"time"
	"strconv"
	"strings"
	"bytes"
	"os"
	"net/http"
	"flag"
	"encoding/gob"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"dat520.github.io/lab3/detector"
	"dat520.github.io/lab6/bank"
	"dat520.github.io/lab6/multipaxos"
	"math/rand"

	color "github.com/fatih/color"
	"github.com/gorilla/websocket"

)

const (
	delta = 6*time.Second
	delta2 = time.Second * 2
)

type Msg struct {
	MsgType   string
	Msg       []byte
	Pid int
	RedirectTo int
}

type DataFromClient struct {
	From        string
	To          string
	Transaction string
	Value       string
}

type IncomingInitialMessage struct {
	Pid int
	Server bool
}

type Process struct {
	ProcessID int
	NumOfProcesses int
	AllProcessIDs []int
	Server bool
	NumOfServers int
	AllServerIDs []int
	ReceivedValue chan bool
	Client bool
	NumOfClients int
	AllClientIDs []int
	ListenConnections map[int]net.Conn
	DialConnections  map[int]net.Conn
	FailureDetectorStruct        *detector.EvtFailureDetector
	LeaderDetectorStruct         *detector.MonLeaderDetector
	Leader                      int
	Subscriber             <-chan int
	proposer              *multipaxos.Proposer
	acceptor              *multipaxos.Acceptor
	learner               *multipaxos.Learner
	Wg                   sync.WaitGroup
	CurRound  int
	BufferedValues        map[int]*multipaxos.DecidedValue
	AccountsList          map[int]*bank.Account
	MainServer       int
	MaxSequence   int
	CatchTimeOut  bool
	TimeoutSignal *time.Ticker
	Value          multipaxos.Value
	Start            time.Time
	End              time.Time
	Stop          		chan bool
	TimeOfOperations []float64
	NumOfOperations  int

	WebSocketConns        *websocket.Conn   //new
	BridgeChannel  chan multipaxos.Response
}

var (
	addrs = []*string{
		flag.String("client1", "127.0.0.1:8080", "http service address"),
		flag.String("client2", "127.0.0.1:8888", "http service address"),
	}

	Address = []string{"localhost:1200","localhost:1201", "localhost:1202"}
	wg = sync.WaitGroup{}
	upgrader = websocket.Upgrader{}
	SuspectChangeChan chan int  //new
	RestoreChangeChan chan int	 //new

	ClientSeq int
	OperationValue int
	AmountValue   int
	AccountNumber int
	AutomaticMode bool
	Message multipaxos.Value
	TransferFrom bank.Account
	TransferTo bank.Account
)

func NewProcess() *Process {
	P := &Process{}
	P.NumOfProcesses = 3
	P.NumOfServers = 3
	P.NumOfClients = 2
	P.MainServer = -1
	P.CatchTimeOut = false
	P.MaxSequence = 0
	P.NumOfOperations = 0


	P.AllProcessIDs = []int{0,1,2}
	P.AllServerIDs = []int{0,1,2}
	P.AllClientIDs = []int{3,4}

	P.Subscriber = make(<-chan int, 10)

	P.ListenConnections = make(map[int]net.Conn)
	P.DialConnections = make(map[int]net.Conn)


	P.Leader = 0
	P.TimeOfOperations = make([]float64, 0)

	for {   // Ask for the process ID
		P.ProcessID = GetIntFromTerminal("Specify the ID of the process ")
		if P.ProcessID < 0 || P.ProcessID > P.NumOfProcesses-1 {
			fmt.Println("This ID is not valid")
		} else {
			break
		}
	}
	var ServerOrClient int
	for {   //Determine wheter the process is a server or a client
		ServerOrClient = GetIntFromTerminal("Press 0 for server and 1 for client")
		if ServerOrClient < 0 || ServerOrClient > 1 {
			fmt.Println("You can only enter 0 or 1")
		}else{
			if ServerOrClient == 0{
				P.Server = true
				P.Client = false
			}else{
				P.Server = false
				P.Client = true
			}
			break
		}
	}
	return P
}
func (P *Process) ListenForConnections() {
	wg.Add(1)
		go func() {
			listener, err := net.Listen("tcp", Address[P.ProcessID])   //(1200,1201,1202,1203,1204,1205)
			CheckError(err)
			for i := 0; i < P.NumOfProcesses; i++ {
				if P.Client && i >= P.NumOfServers{ // makes sure the clients anly have connections with the servers
					continue
				}
				fmt.Println("Port Number : ", P.ProcessID," listening ...")
				conn, AcErr := listener.Accept()  //listens for connections
				CheckError(AcErr)
				incomingMess := IncomingInitialMessage{}
				err2 := json.NewDecoder(conn).Decode(&incomingMess)
				if err2 != nil {
					fmt.Println("The initial message was not received! ", err2.Error())
				}
				remoteProcID := incomingMess.Pid
				if AcErr == nil {
					P.ListenConnections[remoteProcID] = conn
					fmt.Println("Listen.Connected with remote id :", remoteProcID)
				}
			}
			wg.Done()
		}()
}
func (P *Process) DialForConnections() {
	for i := 0  ; i < P.NumOfProcesses; i++ {
		if P.Client && i >= P.NumOfServers{ // makes sure the clients anly have connections with the servers
			continue
		}
		for {
			fmt.Println("Dialing to procces: ", i)
			Conn, err := net.Dial("tcp", Address[i])
			if err == nil {
				P.DialConnections[i] =  Conn
				initialMsg := IncomingInitialMessage{Pid: P.ProcessID}
				err2 := json.NewEncoder(Conn).Encode(initialMsg)
				CheckError(err2)
				fmt.Println("Dialed ", i, " and sent my id ", P.ProcessID)
				break
			}
		}
	}
}
func (P *Process) SendMessageToAllServers(M Msg) {
	for _, key := range P.AllServerIDs{
		if conn, ok := P.DialConnections[key]; ok {
			err := gob.NewEncoder(conn).Encode(&M)
			if err != nil {
				fmt.Println(err.Error())
			}
		}
	}
}
func (P *Process) SendMessageToAllClients(M Msg) {
	for _, key := range P.AllClientIDs{
		if conn, ok := P.DialConnections[key]; ok {
			err := gob.NewEncoder(conn).Encode(&M)
			if err != nil {
				fmt.Println(err.Error())
			}
		}
	}
}
func (P *Process) SendMessageToId(M Msg, id int) {      // sends a messege to a connection specified by the id
	for _, key := range P.AllProcessIDs{
		if id == key {
			if conn, ok := P.DialConnections[key]; ok {
				err1 := gob.NewEncoder(conn).Encode(&M)
				if err1 != nil {
					fmt.Println(err1.Error())
				}
			}
		}
	}
}
func (P *Process) UpdateSubscribe() {  //wait for some write in the subscriber channel in order to print out the new leader
	for {
		time.Sleep(time.Millisecond * 500)
		select {
		case gotLeader := <-P.Subscriber:
			P.Leader = gotLeader
			// Got publication, change the leader
			fmt.Println("------ NEW LEADER: ", P.Leader, "------")
			P.WebSocket()
		}
	}
}
func (P *Process) SendHeartbeats(){ // create a slice of heartbeats to send to the other connected processes
	for {
		for _, key := range P.AllServerIDs{ //creating heartbeat for each connection
			heartbeat := detector.Heartbeat{To: key , From: P.ProcessID, Request: true}
			var network bytes.Buffer
			message := Msg{ Msg: make([]byte,2000),}

			err := gob.NewEncoder(&network).Encode(heartbeat)
			CheckError(err)

			message.Msg = network.Bytes()
			message.MsgType = "Heartbeat"   // type to verify connection
			message.Pid = P.ProcessID

			P.SendMessageToId(message, key)
			time.Sleep(time.Millisecond * 200)
		}
		time.Sleep(time.Millisecond * 2000)
	}
}
func (P *Process) ReceiveMessagesSendReply(hbResponse chan detector.Heartbeat) {
	for _, key := range P.AllServerIDs{
		wg.Add(1)
		go func(key int) {
			for {
				newMsg := Msg{}
				err := gob.NewDecoder(P.ListenConnections[key]).Decode(&newMsg)
				if err != nil {
					if SocketIsClosed(err) {
						P.CloseTheConnection(key, P.ListenConnections[key])
						break
					}
				}

	/*hb*/	if newMsg.MsgType == "Heartbeat" {   // heartbeat type of message
					var heartbeat detector.Heartbeat
					buff:=bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&heartbeat)
					CheckError(err)
					P.FailureDetectorStruct.DeliverHeartbeat(heartbeat)
					if heartbeat.Request == true {
				     	select {
						case hbResp := <-hbResponse:
							MsgReply:= Msg{}
							var network bytes.Buffer
							err := gob.NewEncoder(&network).Encode(hbResp)
							if err != nil {
								fmt.Println(err.Error())
							}
							MsgReply.MsgType = "Heartbeat"
							MsgReply.Msg = network.Bytes()
							err1 := gob.NewEncoder(P.DialConnections[key]).Encode(&MsgReply)
							if err1 != nil {
								fmt.Println(err1.Error())
							}
						}
					}
				}

/*CV*/		if newMsg.MsgType == "ClientValue" {    //Handle the incoming client value
					var value multipaxos.Value
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&value)
					CheckError(err)

					RecieveColor := color.New(color.Bold, color.FgBlue).PrintlnFunc()
					RecieveColor("Recieved value:", value, " from process :", newMsg.Pid )

					if P.Leader == -1 {  //if there is no leader chosen yet
						continue
					}
					if P.ProcessID == P.Leader{
						P.proposer.DeliverClientValue(value)
					}else{
						MsgReply := Msg{}
						var network bytes.Buffer
						err := gob.NewEncoder(&network).Encode(value)
						if err != nil {
							fmt.Println(err.Error())
						}
						MsgReply.MsgType = "Redirect"
						MsgReply.Msg = network.Bytes()
						MsgReply.Pid = P.ProcessID
						MsgReply.RedirectTo = P.Leader

						P.SendMessageToId(MsgReply, newMsg.Pid)

					}
				}

/*PRP*/		if newMsg.MsgType == "Prepare" {
					var prepare multipaxos.Prepare
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&prepare)
					CheckError(err)
					P.acceptor.DeliverPrepare(prepare)
				}


				if newMsg.MsgType == "Promise" {
					var promise multipaxos.Promise
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&promise)
					CheckError(err)
					P.proposer.DeliverPromise(promise)
			 	}

				if newMsg.MsgType == "Accept" {
					var accept multipaxos.Accept
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&accept)
					CheckError(err)
					P.acceptor.DeliverAccept(accept)
			 	}

				if newMsg.MsgType == "Learn" {
					var learn multipaxos.Learn
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&learn)
					CheckError(err)
					P.learner.DeliverLearn(learn)
			 	}
			}
			wg.Done()
		}(key)
	}
}
func (P *Process) PaxosMethod(prepareOut chan multipaxos.Prepare,acceptOut chan multipaxos.Accept, promiseOut chan multipaxos.Promise,learnOut chan multipaxos.Learn, decidedOut chan multipaxos.DecidedValue) {
	for {
		select{
			case prp := <-prepareOut:
				var network bytes.Buffer
				err := gob.NewEncoder(&network).Encode(prp)
				CheckError(err)
				MsgReply := Msg{ MsgType : "Prepare", Msg : network.Bytes(), Pid : P.ProcessID}
				P.SendMessageToAllServers(MsgReply) //broadcast the prepare message

			case prm := <-promiseOut:
				var network bytes.Buffer
				err := gob.NewEncoder(&network).Encode(prm)
				CheckError(err)
				MsgReply := Msg{MsgType : "Promise", Msg : network.Bytes(), Pid : P.ProcessID }
				P.SendMessageToId(MsgReply, prm.To) //Send the promise only to the sender of propose m

			case acc := <-acceptOut:

				var network bytes.Buffer
				err := gob.NewEncoder(&network).Encode(acc)
				CheckError(err)
				MsgReply := Msg{MsgType : "Accept", Msg : network.Bytes(), Pid : P.ProcessID }
				P.SendMessageToAllServers(MsgReply)             //Send the accept message to everyone

			case lrn := <-learnOut:
				var network bytes.Buffer
				err := gob.NewEncoder(&network).Encode(lrn)
				CheckError(err)
				MsgReply := Msg{MsgType : "Learn", Msg : network.Bytes(), Pid : P.ProcessID}
				P.SendMessageToAllServers(MsgReply) //broadcast the learn message

			case decidedVal := <-decidedOut:
			// handle the decided value (all the servers, not only the leader)
				DecidedColor := color.New(color.Bold, color.FgRed).PrintlnFunc()
				DecidedColor("Multipaxos: Got the Decided Value ", decidedVal )
				Message := P.HandleDecideValue(decidedVal)

				if P.ProcessID == P.Leader {
					P.BridgeChannel <- Message
				}
		}
	}
}
func (P *Process) HandleDecideValue(decidedVal multipaxos.DecidedValue) (msg multipaxos.Response) {
	fmt.Println("Im in HandleDecideValue")
	adu := P.proposer.GetADU()
	if decidedVal.SlotID > adu+1 {
		P.BufferedValues[int(adu)+1] = &decidedVal
		return
	}

	if !decidedVal.Value.Noop {

		// apply transaction from value to account
		var TransactionResult bank.TransactionResult

		if decidedVal.Value.Txn.Op == 3 {
			Trans1:=bank.Transaction{Op:2, From:decidedVal.Value.Txn.From, To:decidedVal.Value.Txn.To, Amount: decidedVal.Value.Txn.Amount,}
			TransactionResult = P.AccountsList[decidedVal.Value.Txn.From.Number].Process(Trans1)

			Trans1str := fmt.Sprintf("%v", Trans1)

			if TransactionResult.ErrorString == Trans1str || TransactionResult.ErrorString == "" {

				Trans2:=bank.Transaction{Op:1, From:decidedVal.Value.Txn.From, To:decidedVal.Value.Txn.To, Amount: decidedVal.Value.Txn.Amount,}

				if P.AccountsList[decidedVal.Value.Txn.To.Number] == nil {
					P.AccountsList[decidedVal.Value.Txn.To.Number] = &bank.Account{Number: decidedVal.Value.Txn.To.Number, Balance: 0}
				}
				P.AccountsList[decidedVal.Value.Txn.To.Number].Process(Trans2)
			}
		} else {

			if P.AccountsList[decidedVal.Value.AccountNum] == nil {
				// create and store new account with a balance of zero
				P.AccountsList[decidedVal.Value.AccountNum] = &bank.Account{Number: decidedVal.Value.AccountNum, Balance: 0}
			}
			// for the all other transactions just call a simple process function
			TransactionResult = P.AccountsList[decidedVal.Value.AccountNum].Process(decidedVal.Value.Txn)
		}


		// create response with appropriate transaction result, client id and client seq
		Response := multipaxos.Response{ClientID: decidedVal.Value.ClientID, ClientSeq: decidedVal.Value.ClientSeq, TxnRes: TransactionResult}

		if P.ProcessID == P.Leader {
			msg = Response
		}
	}
	// increment adu by 1 (increment decided slot for proposer)
	P.proposer.IncrementAllDecidedUpTo()
	if P.BufferedValues[int(adu)+1] != nil {
		msg = P.HandleDecideValue(*P.BufferedValues[int(adu)+1])
	}
	return msg
}

func (P *Process) WebSocket() {
	// only the leader communicate with the client(s)
	if P.ProcessID == P.Leader {
		http.HandleFunc("/ws" + strconv.Itoa(P.ProcessID), P.ServeWs)

		for addrInd := 0; addrInd < len(addrs); addrInd++ {
			go func(addrInd int) {
				log.Fatal(http.ListenAndServe(*addrs[addrInd], nil)) // the leader listen to all the address saved in the addrs variable
			}(addrInd)
		}
	}
}
// function that handle the communication between server and client
func (P *Process) ServeWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	P.WebSocketConns = ws

	for {
		var data DataFromClient

		err := ws.ReadJSON(&data) // read JSON data
		if err != nil {
			fmt.Println(data)

			fmt.Println(err)
			break
		}

		var mainErr error
		go func() {
			msg := P.HandleMessage(data)
			mainErr = ws.WriteJSON(msg) // send the JSON data
		}()

		if mainErr != nil {
			fmt.Println(mainErr)
			break
		}
	}
}
// function that load the html file of the client
func (P *Process) ServeHome(w http.ResponseWriter, r *http.Request) {
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
func (P *Process) HandleMessage(data DataFromClient) (response multipaxos.Response) {
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

		if P.AccountsList[accountNumTo] == nil {
			// if there isn't an account to transfer the money, initialize it
			P.AccountsList[accountNumTo] = &bank.Account{Number: accountNumTo, Balance: 0}
		}
		txn.To = *P.AccountsList[accountNumTo]
	}

	txn.Amount, _ = strconv.Atoi(data.Value)
	message.AccountNum, _ = strconv.Atoi(data.From)

	if P.AccountsList[message.AccountNum] == nil {
		P.AccountsList[message.AccountNum] = &bank.Account{Number: message.AccountNum, Balance: 0}
	}
	txn.From = *P.AccountsList[message.AccountNum]

	message.Txn = txn
	message.ClientID = data.From
	message.ClientSeq = ClientSeq

	log.WithFields(log.Fields{"msg": message}).Info("RECEIVING VALUE FROM CLIENT:")
	if P.ProcessID == P.Leader {
		P.proposer.DeliverClientValue(message)
	}

	for {
		select {
		case msg := <-P.BridgeChannel: // wait for the result from Paxos and then increase the sequence
			ClientSeq += 1
			return msg
		}
	}

}


func main(){

	Proc:= NewProcess()
	Proc.ListenForConnections()
	Proc.DialForConnections()
	wg.Wait()
	Proc.PrintConnectionInformation()

	wg.Add(2)
	if Proc.Server {

		Proc.BufferedValues = make(map[int]*multipaxos.DecidedValue)
		Proc.AccountsList = make(map[int]*bank.Account)

		// add the leader Detector Struct
		Proc.LeaderDetectorStruct = detector.NewMonLeaderDetector(Proc.AllServerIDs) //Call the Leader detector only on servers
		Proc.Leader = Proc.LeaderDetectorStruct.Leader()
		fmt.Println("------LEADER IS PROCESS: ", Proc.Leader,"------"); fmt.Println("")

		//Subscribe for leader changes
		Proc.Subscriber = Proc.LeaderDetectorStruct.Subscribe()
		go Proc.UpdateSubscribe()    // wait to update the subscriber

		Proc.BridgeChannel = make(chan multipaxos.Response, 20)

		// Add the eventual failure detector
		hbResponse := make(chan detector.Heartbeat, 10) //channel to communicate with fd
		Proc.FailureDetectorStruct = detector.NewEvtFailureDetector(Proc.ProcessID, Proc.AllServerIDs, Proc.LeaderDetectorStruct, delta, hbResponse)
		Proc.FailureDetectorStruct.Start()

		//Paxos Implementation
		prepareOut := make(chan multipaxos.Prepare, 10)
		acceptOut := make(chan multipaxos.Accept, 10)
		promiseOut := make(chan multipaxos.Promise, 10)
		learnOut := make(chan multipaxos.Learn, 10)
		decidedOut := make(chan multipaxos.DecidedValue, 10)

		wg.Add(4)
		Proc.ReceiveMessagesSendReply(hbResponse)


		go Proc.PaxosMethod(prepareOut, acceptOut, promiseOut, learnOut, decidedOut)

		Proc.proposer = multipaxos.NewProposer(Proc.ProcessID, Proc.NumOfServers, -1, Proc.LeaderDetectorStruct, prepareOut, acceptOut)
		Proc.proposer.Start()
		Proc.acceptor = multipaxos.NewAcceptor(Proc.ProcessID, promiseOut, learnOut)
		Proc.acceptor.Start()
		Proc.learner = multipaxos.NewLearner(Proc.ProcessID, Proc.NumOfServers, decidedOut)
		Proc.learner.Start()

		go Proc.SendHeartbeats()

		http.HandleFunc("/", Proc.ServeHome)
		Proc.WebSocket() // start the web socket; the leader is listening

		wg.Wait()
	}

	wg.Wait()
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func (P *Process) StopGenerator() {
	P.Stop <- true // stop the automatic send
}
func (P *Process) StartGenerator() {
	P.ReceivedValue <- true
}
func (P *Process) ResetTicker() {
    P.TimeoutSignal = time.NewTicker(time.Second * 10)
}
func RandomFromSlice(slice []int) int {
    rand.Seed(time.Now().Unix())
    message := slice[rand.Intn(len(slice))]
    return message
}
func (P *Process) PrintConnectionInformation(){
		fmt.Println("------CONNECTIONS ESTABLISHED------")
		fmt.Println("Listen conns : ", P.ListenConnections)
		fmt.Println("Dial conns : ",P.DialConnections)
}
func SocketIsClosed(err error) bool {
	if strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "forcibly closed by the remote host") {
		return true
	}
	return false
}
func (P *Process) CloseTheConnection(index int, conn net.Conn) {
	conn.Close()
	delete(P.ListenConnections, index)
	delete(P.DialConnections, index)

	P.AllProcessIDs = append(P.AllProcessIDs[:index], P.AllProcessIDs[index+1:]...)
	P.AllServerIDs = append(P.AllServerIDs[:index], P.AllServerIDs[index+1:]...)

	if P.Leader == index{  //if the leader died exclude him from the leadership
		P.Leader = -1
	}

	IamServer := PresenceInSlice(P.AllServerIDs, P.ProcessID)
	if IamServer{
		fmt.Println("Listen conns after disconnect", P.ListenConnections)
		fmt.Println("Dial conns after disconnect", P.DialConnections)
		fmt.Println("All process ids :",P.AllProcessIDs)
		fmt.Println("All server ids :",P.AllServerIDs)
		fmt.Println("All client ids :",P.AllClientIDs)

		fmt.Println("******CLOSED CONNECTION WITH PROCESS: ", index,"******")
	}


}
func PresenceInSlice(slice []int, id int) bool{
	output := false
	for _, key := range slice{
		if id == key {
			output = true
		}
	}
	return output
}
func CheckError(err error) {
    if err != nil {
        fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
    }
}
func TerminalInput(message string) string {
	var input string
	fmt.Print(message, ": ")
	fmt.Scanln(&input)
	return input
}
func GetIntFromTerminal(message string) int {
	s := TerminalInput(message)
	x, _ := strconv.ParseInt(s, 10, 32)
	return int(x)
}
