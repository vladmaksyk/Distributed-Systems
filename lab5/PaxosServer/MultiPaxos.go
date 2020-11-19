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
	"encoding/gob"
	"encoding/json"
	"dat520.github.io/lab3/detector"
	"dat520.github.io/lab5/bank"
	"dat520.github.io/lab5/multipaxos"
	"math/rand"
	"math"
	"github.com/stats"
	color "github.com/fatih/color"

)

var Address = []string{"localhost:1200", "localhost:1201", "localhost:1202","localhost:1203","localhost:1204","localhost:1205"}
//var Address = []string{"172.18.0.0/1", "172.18.0.1/1", "172.18.0.2/1","172.18.0.3/1","172.18.0.4/1"}

var wg = sync.WaitGroup{}

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


	Client bool
	NumOfClients int
	AllClientIDs []int

	ListenConnections map[int]net.Conn
	DialConnections  map[int]net.Conn

	FailureDetectorStruct        *detector.EvtFailureDetector
	LeaderDetectorStruct         *detector.MonLeaderDetector
	Leader                      int
	Subscriber             <-chan int
	BreakOut						chan bool

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
	TimeOfOperations []float64


}

var (
	OperationValue int
	AmountValue   int
	AccountNumber int
)

func NewProcess() *Process {
	P := &Process{}
	P.NumOfProcesses = 6
	P.NumOfServers = 3
	P.NumOfClients = 3
	P.MainServer = -1
	P.CatchTimeOut = false
	P.MaxSequence = 0

	P.AllProcessIDs = []int{0,1,2,3,4,5}
	P.AllServerIDs = []int{0,1,2}
	P.AllClientIDs = []int{3,4,5}

	P.Subscriber = make(<-chan int, 1)
	P.ListenConnections = make(map[int]net.Conn)
	P.DialConnections = make(map[int]net.Conn)
	P.BreakOut = make(chan bool)

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
			listener, err := net.Listen("tcp", Address[P.ProcessID])   //(1200,1201,1202,1203,1204)
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
			err := gob.NewEncoder(P.DialConnections[key]).Encode(&M)
			if err != nil {
				fmt.Println(err.Error())
			}
	}
}
func (P *Process) SendMessageToAllClients(M Msg) {
	for _, key := range P.AllClientIDs{
			err := gob.NewEncoder(P.DialConnections[key]).Encode(&M)
			if err != nil {
				fmt.Println(err.Error())
			}
	}
}
func (P *Process) SendMessageToId(M Msg, id int) {      // sends a messege to a connection specified by the id
	for _, key := range P.AllProcessIDs{
		if id == key {
			err1 := gob.NewEncoder(P.DialConnections[key]).Encode(&M)
			if err1 != nil {
				fmt.Println(err1.Error())
			}
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
			time.Sleep(time.Millisecond * 300)
		}
		time.Sleep(time.Millisecond * 1000)
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
			}
	}
}
func (P *Process) ReceiveMessagesSendReply(hbResponse chan detector.Heartbeat) {
	for _, key := range P.AllProcessIDs{
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
							MsgReply:=Msg{}
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
					fmt.Println("Recieved value:", value, " from process :", newMsg.Pid ); fmt.Println("")

					if P.Leader == -1 {  //if there is no leader chosen yet
						continue
					}
					if P.ProcessID == P.Leader{
						P.proposer.DeliverClientValue(value)
						fmt.Println("Sent value to proposer" ); fmt.Println("")
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
					fmt.Println("Got the prepare message:", prepare, " from process :", newMsg.Pid ); fmt.Println("")
					P.acceptor.DeliverPrepare(prepare)
				}


				if newMsg.MsgType == "Promise" {
					var promise multipaxos.Promise
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&promise)
					CheckError(err)
					fmt.Println("Got the promise message:", promise, " from process :", newMsg.Pid ); fmt.Println("")
					P.proposer.DeliverPromise(promise)
			 	}

				if newMsg.MsgType == "Accept" {
					var accept multipaxos.Accept
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&accept)
					CheckError(err)
					fmt.Println("Got the accept message:", accept, " from process :", newMsg.Pid ); fmt.Println("")
					P.acceptor.DeliverAccept(accept)
			 	}

				if newMsg.MsgType == "Learn" {
					var learn multipaxos.Learn
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&learn)
					CheckError(err)
					fmt.Println("Got the learn message:", learn, " from process :", newMsg.Pid )
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
				MsgReply := Msg{}
				var network bytes.Buffer
				err := gob.NewEncoder(&network).Encode(prp)
				if err != nil {
					fmt.Println(err.Error())
				}
				MsgReply.MsgType = "Prepare"
				MsgReply.Msg = network.Bytes()
				MsgReply.Pid = P.ProcessID
				P.SendMessageToAllServers(MsgReply) //broadcast the prepare message

			case prm := <-promiseOut:
				MsgReply := Msg{}
				var network bytes.Buffer
				err := gob.NewEncoder(&network).Encode(prm)
				if err != nil {
					fmt.Println(err.Error())
				}
				MsgReply.MsgType = "Promise"
				MsgReply.Msg = network.Bytes()
				MsgReply.Pid = P.ProcessID
				P.SendMessageToId(MsgReply, prm.To) //Send the promise only to the sender of propose m
				time.Sleep(300 * time.Millisecond)

			case acc := <-acceptOut:
				MsgReply := Msg{}
				var network bytes.Buffer
				err := gob.NewEncoder(&network).Encode(acc)
				if err != nil {
					fmt.Println(err.Error())
				}
				MsgReply.MsgType = "Accept"
				MsgReply.Msg = network.Bytes()
				MsgReply.Pid = P.ProcessID
				P.SendMessageToAllServers(MsgReply)             //Send the accept message to everyone
				fmt.Println("Sent the accept message to all"); fmt.Println("")

			case lrn := <-learnOut:
				MsgReply := Msg{}
				var network bytes.Buffer
				err := gob.NewEncoder(&network).Encode(lrn)
				if err != nil {
					fmt.Println(err.Error())
				}
				MsgReply.MsgType = "Learn"
				MsgReply.Msg = network.Bytes()
				MsgReply.Pid = P.ProcessID
				P.SendMessageToAllServers(MsgReply) //broadcast the learn message

			case decidedVal := <-decidedOut:
				fmt.Println("--Got the decided out value"); fmt.Println("")
			// handle the decided value (all the servers, not only the leader)
				P.HandleDecideValue(decidedVal)
		}
	}
}

func (P *Process) HandleDecideValue(decidedVal multipaxos.DecidedValue) {
	adu := P.proposer.GetADU()
	if decidedVal.SlotID > adu+1 {
		P.BufferedValues[int(adu)+1] = &decidedVal
		return
	}
	if !decidedVal.Value.Noop {
		if P.AccountsList[decidedVal.Value.AccountNum] == nil {
			// create and store new account with a balance of zero
			P.AccountsList[decidedVal.Value.AccountNum] = &bank.Account{Number: decidedVal.Value.AccountNum, Balance: 0}
		}
		// apply transaction from value to account
		TransactionResult := P.AccountsList[decidedVal.Value.AccountNum].Process(decidedVal.Value.Txn)

		if P.ProcessID == P.Leader {
			// create response with appropriate transaction result, client id and client seq
			Response := multipaxos.Response{ClientID: decidedVal.Value.ClientID, ClientSeq: decidedVal.Value.ClientSeq, TxnRes: TransactionResult}
			MsgReply := Msg{}
			var network bytes.Buffer
			err := gob.NewEncoder(&network).Encode(Response)
			if err != nil {
				fmt.Println(err.Error())
			}
			MsgReply.MsgType = "ResponseToClient"
			MsgReply.Msg = network.Bytes()
			MsgReply.Pid = P.ProcessID
			ClientID, _ := strconv.Atoi(decidedVal.Value.ClientID)
			P.SendMessageToId(MsgReply, ClientID)

		}
	}
	// increment adu by 1 (increment decided slot for proposer)
	P.proposer.IncrementAllDecidedUpTo()
	if P.BufferedValues[int(adu)+1] != nil {
		P.HandleDecideValue(*P.BufferedValues[int(adu)+1])
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

		//subscribe for leader changes
		Proc.Subscriber = Proc.LeaderDetectorStruct.Subscribe()
		go Proc.UpdateSubscribe()    // wait to update the subscriber

		// Add the eventual failure detector
		hbResponse := make(chan detector.Heartbeat, 8) //channel to communicate with fd
		Proc.FailureDetectorStruct = detector.NewEvtFailureDetector(Proc.ProcessID, Proc.AllServerIDs, Proc.LeaderDetectorStruct, delta, hbResponse)
		Proc.FailureDetectorStruct.Start()

		// --- paxos implementation
		prepareOut := make(chan multipaxos.Prepare, 8)
		acceptOut := make(chan multipaxos.Accept, 8)
		promiseOut := make(chan multipaxos.Promise, 8)
		learnOut := make(chan multipaxos.Learn, 8)
		decidedOut := make(chan multipaxos.DecidedValue, 8)

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
		wg.Wait()
	}

	if Proc.Client {


		// choose between manual and automatic mode
		Mode := TerminalInput("Manual mode (m) or Automatic mode (a)? ")

		if Proc.MainServer == -1 { // if the main server was not chosen
			Proc.MainServer = RandomFromSlice(Proc.AllServerIDs)
			fmt.Println("Current Main Server:",Proc.MainServer )
		}

		wg.Add(2)
		go Proc.GetTheValue()

		if Mode == "m" {
			go Proc.SendValueManual()
		} else if Mode == "a" {
			go Proc.SendValueAutomatic()
		}

		go Proc.WaitForTimeout()

		wg.Wait()
	}
	wg.Wait()
}

func (P *Process) WaitForTimeout() {
	P.TimeoutSignal = time.NewTicker(time.Second * 10)
	Warning := color.New(color.FgRed).PrintfFunc()
	for {
		select{
		case <- P.TimeoutSignal.C:
			if P.CatchTimeOut {
				Warning("*****Got Timout*****"); fmt.Println("")
				if _, ok := P.DialConnections[P.MainServer]; !ok {   //Check if the MainServer is alive, if not then choose another server randomly
					Warning("Main Server is not alive, choosing a new server randlomy "); fmt.Println("")
					P.MainServer = RandomFromSlice(P.AllServerIDs)
				}
				go P.SendValue()  //Resend the value
			}
		}
	}
}

func (P *Process) SendValueManual() {
	for {
		if !P.CatchTimeOut {
			time.Sleep(time.Millisecond * 100)

			OperationValue = GetIntFromTerminal("Specify the operation (0: Balance, 1: Deposit, 2: Withdrawal) ")
			AccountNumber = GetIntFromTerminal("Insert the account number ")
			AmountValue = 0
			if OperationValue > 0 {
				AmountValue = GetIntFromTerminal("Insert the amount ")
			}
			ClientID := strconv.Itoa(P.ProcessID)
			transaction := bank.Transaction{Op: bank.Operation(OperationValue), Amount: AmountValue}

			P.Value = multipaxos.Value{ClientID: ClientID, ClientSeq: P.MaxSequence, AccountNum: AccountNumber, Txn: transaction}

			P.CatchTimeOut = true
			go P.SendValue()
		}
	}
}

func (P *Process) SendValueAutomatic() {
	startTT := time.Now()
	for i := 0; i < 501; {
		if !P.CatchTimeOut {
			time.Sleep(time.Millisecond * 10)
			P.Start = time.Now()
			OperationValue = rand.Intn(3)
			AccountNumber = rand.Intn(1000)
			AmountValue = 0
			if OperationValue > 0 {
				AmountValue = rand.Intn(10000)
			}
			ClientID := strconv.Itoa(P.ProcessID)
			transaction := bank.Transaction{Op: bank.Operation(OperationValue), Amount: AmountValue}
			P.Value = multipaxos.Value{ClientID: ClientID, ClientSeq: P.MaxSequence, AccountNum: AccountNumber, Txn: transaction}

			P.CatchTimeOut = true
			go P.SendValue()
			i++
		}else{
			time.Sleep(time.Millisecond * 300)
		}
	}
	endTT := time.Now()
	//Print out the stats
	time.Sleep(time.Second * 1)
	mean, _ := stats.Mean(P.TimeOfOperations)
	minimum, _ := stats.Min(P.TimeOfOperations)
	maximum, _ := stats.Max(P.TimeOfOperations)
	median, _ := stats.Median(P.TimeOfOperations)
	variance, _ := stats.Variance(P.TimeOfOperations)
	stddev := math.Sqrt(variance)
	percentile, _ := stats.Percentile(P.TimeOfOperations, 99)

	TotalTime := endTT.Sub(startTT).Seconds()

	Info := color.New(color.Bold, color.FgRed).PrintlnFunc()
	Info("Mean in seconds: ", mean )
	Info("Minimum in seconds:", minimum )
	Info("Maximum in seconds:", maximum )
	Info("Median in seconds: ", median )
	Info("Standard deviation in seconds: ", stddev )
	Info("99 Percentile in seconds: ", percentile )
	Info("Total time for 500 Operations in seconds: ", TotalTime )
}

func (P *Process) SendValue(){
	P.TimeoutSignal = time.NewTicker(time.Second * 10)
	var network bytes.Buffer
	message := Msg{ Msg: make([]byte,2000),}
	err := gob.NewEncoder(&network).Encode(P.Value)
	CheckError(err)
	message.Msg = network.Bytes()
	message.MsgType = "ClientValue"   // type of client value
	message.Pid = P.ProcessID
	P.SendMessageToId(message, P.MainServer)
	fmt.Println("Sent Value :", P.Value, " To Server : ", P.MainServer)
}

func  (P *Process) GetTheValue() {
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
/*RV*/		if newMsg.MsgType == "ResponseToClient" {    //Handle the incoming client value
					var response multipaxos.Response
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&response)
					CheckError(err)

					ResponseColor := color.New(color.Bold, color.FgGreen).PrintlnFunc()
					ResponseColor("Recieved response:", response, " from server :", newMsg.Pid ); fmt.Println("")

					P.CatchTimeOut = false   //signal for the client to ignore the timout since we got the value
					P.MaxSequence++

					P.End = time.Now()
					P.TimeOfOperations = append(P.TimeOfOperations, P.End.Sub(P.Start).Seconds())

				}
/*R	*/		if newMsg.MsgType == "Redirect" {    //Handle the redirect message
					P.MainServer = newMsg.RedirectTo
					var value multipaxos.Value
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&value)
					CheckError(err)
					fmt.Println("Recieved redirect to server :", newMsg.RedirectTo, " from server :", newMsg.Pid, "with value : ", value )
					fmt.Println("The main server is now :", P.MainServer)

					var network bytes.Buffer                // and send the value to the correct server according to the redirect message
					message := Msg{ Msg: make([]byte,2000),}
					err := gob.NewEncoder(&network).Encode(value)
					CheckError(err)
					message.Msg = network.Bytes()
					message.MsgType = "ClientValue"   // type of client value
					message.Pid = P.ProcessID
					P.SendMessageToId(message, P.MainServer)
					fmt.Println("Sent Value :", value, " To Server : ", P.MainServer)

				}
			}
			wg.Done()
		}(key)
	}
}



///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
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
