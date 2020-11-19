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
	"dat520.github.io/lab4/singlepaxos"

)

var Address = []string{"localhost:1200", "localhost:1201", "localhost:1202","localhost:1203","localhost:1204"}
var wg = sync.WaitGroup{}
const (
	delta = 6*time.Second
)

type Msg struct {
	MsgType   string
	Msg       []byte
	Pid int
}

type IncomingInitialMessage struct {
	Pid int
	Server bool
}

type Process struct {
	ProcessID int
	NumOfProcesses int
	AllProcessIDs []int
	NumberOFConnectedProcesses int

	Server bool
	AllServerIDs []int
	NumOfServers int
	ListenConnections map[int]net.Conn
	DialConnections  map[int]net.Conn

	Client bool
	AllClientIDs []int
	NumOfClients int
	ListenConnectionsClient map[int]net.Conn
	DialConnenctionsClient  map[int]net.Conn

	Connections          []net.Conn
	Conns						map[int]net.Conn

	FailureDetectorStruct        *detector.EvtFailureDetector
	LeaderDetectorStruct         *detector.MonLeaderDetector
	Leader                      int
	Subscriber             <-chan int

	proposer              *singlepaxos.Proposer
	acceptor              *singlepaxos.Acceptor
	learner               *singlepaxos.Learner

	Wg                   sync.WaitGroup

	CurRound  int

}
func NewProcess() *Process {
	P := &Process{}
	P.NumberOFConnectedProcesses = 0
	P.NumOfProcesses = 5
	P.NumOfServers = 3
	P.NumOfClients = 2

	P.AllProcessIDs = []int{0,1,2,3,4}
	P.AllServerIDs = []int{0,1,2}
	P.AllClientIDs = []int{3,4}
	P.Subscriber = make(<-chan int, 1)
	P.Conns = make(map[int]net.Conn)
	P.ListenConnections = make(map[int]net.Conn)
	P.DialConnections = make(map[int]net.Conn)

	P.ListenConnectionsClient = make(map[int]net.Conn)
	P.DialConnenctionsClient = make(map[int]net.Conn)

	P.Leader = 0

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
			fmt.Println("You might enter 0 or 1")
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
			//fmt.Println("Sent :" , heartbeat)
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
				//P.ProcessID = gotLeader
				P.Leader = gotLeader
				// Got publication, change the leader
				fmt.Println("------ NEW LEADER: ", P.Leader, "------")
			}
	}
}

func (P *Process) ReceiveMessagesSendReply(hbResponse chan detector.Heartbeat, prepareOut chan singlepaxos.Prepare, promiseOut chan singlepaxos.Promise, acceptOut chan singlepaxos.Accept,  learnOut chan singlepaxos.Learn, valueOut chan singlepaxos.Value ) {

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
				//fmt.Println("Listening on connection", key)

	/*hb*/	if newMsg.MsgType == "Heartbeat" {   // heartbeat type of message
					var heartbeat detector.Heartbeat
					buff:=bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&heartbeat)
					CheckError(err)
					//fmt.Println("Received :",heartbeat )
					// if heartbeat.Request == false {
					// 	fmt.Println("Received :",heartbeat )
					// }
					P.FailureDetectorStruct.DeliverHeartbeat(heartbeat)
					if heartbeat.Request == true {
				     	select {
						case hbResp := <-hbResponse:
							//fmt.Println("received from hbResponse: ", hbResp)
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
					var value singlepaxos.Value
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&value)
					CheckError(err)
					fmt.Println("Recieved value:", value, " from process :", newMsg.Pid )
					P.proposer.DeliverClientValue(value)
				}

/*PRP*/		if newMsg.MsgType == "Prepare" {
					var prepare singlepaxos.Prepare
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&prepare)
					CheckError(err)
					fmt.Println("Got the prepare message:", prepare, " from process :", newMsg.Pid )
					P.acceptor.DeliverPrepare(prepare)
				}


				if newMsg.MsgType == "Promise" {
					var promise singlepaxos.Promise
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&promise)
					CheckError(err)
					fmt.Println("Got the promise message:", promise, " from process :", newMsg.Pid )
					P.proposer.DeliverPromise(promise)
			 	}

				if newMsg.MsgType == "Accept" {
					var accept singlepaxos.Accept
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&accept)
					CheckError(err)
					fmt.Println("Got the accept message:", accept, " from process :", newMsg.Pid )
					P.acceptor.DeliverAccept(accept)
			 	}

				if newMsg.MsgType == "Learn" {
					var learn singlepaxos.Learn
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

func (P *Process) PaxosMethod(prepareOut chan singlepaxos.Prepare,
	acceptOut chan singlepaxos.Accept, promiseOut chan singlepaxos.Promise,
	learnOut chan singlepaxos.Learn, valueOut chan singlepaxos.Value) {
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
				fmt.Println("Sent the accept message to all")

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

			case val := <-valueOut:
				fmt.Println("Got the value from learner :", val)
				MsgReply := Msg{}
				var network bytes.Buffer
				err := gob.NewEncoder(&network).Encode(val)
				if err != nil {
					fmt.Println(err.Error())
				}

				MsgReply.MsgType = "ServerValue"
				MsgReply.Msg = network.Bytes()
				MsgReply.Pid = P.ProcessID
				//send the value to the clients
				if P.ProcessID == P.Leader {
					P.SendMessageToAllClients(MsgReply)
					fmt.Println("Sent value to all clients")
				}
		}
	}
}

func main(){
	Proc:= NewProcess()
	Proc.ListenForConnections()
	Proc.DialForConnections()
	wg.Wait()

	fmt.Println("------CONNECTIONS ESTABLISHED------")
	fmt.Println("Listen conns : ", Proc.ListenConnections)
	fmt.Println("Dial conns : ",Proc.DialConnections)

	wg.Add(2)
	if Proc.Server {
		// add the leader Detector Struct
		Proc.LeaderDetectorStruct = detector.NewMonLeaderDetector(Proc.AllServerIDs) //Call the Leader detector only on servers
		Proc.Leader = Proc.LeaderDetectorStruct.Leader()
		fmt.Println("------LEADER IS PROCESS: ", Proc.Leader,"------")

		//subscribe for leader changes
		Proc.Subscriber = Proc.LeaderDetectorStruct.Subscribe()
		go Proc.UpdateSubscribe()    // wait to update the subscriber

		// Add the eventual failure detector
		hbResponse := make(chan detector.Heartbeat, 8) //channel to communicate with fd
		Proc.FailureDetectorStruct = detector.NewEvtFailureDetector(Proc.ProcessID, Proc.AllServerIDs, Proc.LeaderDetectorStruct, delta, hbResponse)
		Proc.FailureDetectorStruct.Start()

		// --- paxos implementation
		prepareOut := make(chan singlepaxos.Prepare, 8)
		acceptOut := make(chan singlepaxos.Accept, 8)
		promiseOut := make(chan singlepaxos.Promise, 8)
		learnOut := make(chan singlepaxos.Learn, 8)
		valueOut := make(chan singlepaxos.Value, 8)

		go Proc.PaxosMethod(prepareOut, acceptOut, promiseOut, learnOut, valueOut)

		Proc.proposer = singlepaxos.NewProposer(Proc.ProcessID, Proc.NumOfServers, Proc.LeaderDetectorStruct, prepareOut, acceptOut)
		Proc.proposer.Start()
		Proc.acceptor = singlepaxos.NewAcceptor(Proc.ProcessID, promiseOut, learnOut)
		Proc.acceptor.Start()
		Proc.learner = singlepaxos.NewLearner(Proc.ProcessID, Proc.NumOfServers, valueOut)
		Proc.learner.Start()

		wg.Add(4)
		Proc.ReceiveMessagesSendReply(hbResponse, prepareOut, promiseOut, acceptOut, learnOut, valueOut)

		go Proc.SendHeartbeats()



		wg.Wait()
	}
	if Proc.Client {
		wg.Add(2)
		go Proc.SendValue()
		go Proc.GetTheValue()
		wg.Wait()
	}
	wg.Wait()

}

func (P *Process) SendValue() {
	for {

		var value singlepaxos.Value
		strValue := TerminalInput("Insert a Value: ")
		value = singlepaxos.Value(strValue)

		for _, key := range P.AllServerIDs{
			var network bytes.Buffer
			message := Msg{ Msg: make([]byte,2000),}
			err := gob.NewEncoder(&network).Encode(value)
			CheckError(err)
			message.Msg = network.Bytes()
			message.MsgType = "ClientValue"   // type of client value
			message.Pid = P.ProcessID
			P.SendMessageToId(message, key)
			fmt.Println("Sent Value :", value, " To Server : ", key)
			time.Sleep(300 * time.Millisecond) // wait to do something else
		}
		time.Sleep(10 * time.Second) // wait to do something else
	}
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

/*SV*/		if newMsg.MsgType == "ServerValue" {    //Handle the incoming client value
					var value singlepaxos.Value
					buff := bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&value)
					CheckError(err)
					fmt.Println("Recieved value:", value, " from server :", newMsg.Pid )
				}
			}
			wg.Done()
		}(key)
	}
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

	fmt.Println("Listen conns after disconnect", P.ListenConnections)
	fmt.Println("Dial conns after disconnect", P.DialConnections)
	fmt.Println("All process ids :",P.AllProcessIDs)
	fmt.Println("All server ids :",P.AllServerIDs)
	fmt.Println("All client ids :",P.AllClientIDs)

	fmt.Println("******CLOSED CONNECTION WITH PROCESS: ", index,"******")
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
