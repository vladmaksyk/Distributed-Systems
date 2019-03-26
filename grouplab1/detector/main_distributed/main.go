package main

import(
	"fmt"
	"net"
	"sync"
	"time"
	"strconv"
	"strings"
	"bytes"
	"bufio"
	"os"
	"encoding/gob"
	"github.com/uis-dat520-s2019/GMaksyk_18/grouplab1/detector"

)

var Address = []string{"localhost:1200", "localhost:1201", "localhost:1202"}
var wg = sync.WaitGroup{}
const (
	delta = 6*time.Second
)

type Msg struct {
	MsgType   int
	Msg       []byte
	Pid int
}

type Process struct {
	ProcessID int
	NumOfProcesses int
	AllProcessIDs []int
	NumberOFConnectedProcesses int

	ListenConnections map[int]net.Conn
	DialConnenctions  map[int]net.Conn
	Connections          []net.Conn
	Conns						map[int]net.Conn

	FailureDetectorStruct        *detector.EvtFailureDetector
	LeaderDetectorStruct         *detector.MonLeaderDetector
	Leader                      int
	Subscriber             <-chan int
	Wg                   sync.WaitGroup

}
func NewProcess() *Process {
	P := &Process{}
	P.NumberOFConnectedProcesses = 0
	P.NumOfProcesses = 3
	P.AllProcessIDs = []int{0,1,2}
	P.Subscriber = make(<-chan int, 1)
	P.Conns = make(map[int]net.Conn)
	P.ListenConnections = make(map[int]net.Conn)
	P.DialConnenctions = make(map[int]net.Conn)
	P.Leader = 0

	for {
		P.ProcessID = GetIntFromTerminal("Specify the ID of the process ")
		if P.ProcessID < 0 || P.ProcessID > P.NumOfProcesses-1 {
			fmt.Println("This ID is not valid")
		} else {
			break
		}
	}
	return P
}
/*func (P *Process) ListenForConnections() {
	P.Wg.Add(1)
	// Incomming Connections Handled In this Go Routine
	go func() {
		listener, err := net.Listen("tcp", Address[P.ProcessID])   //(1200,1201,1202)
		CheckError(err)
		for i := 0; i < P.ProcessID; i++ {
			fmt.Println("Port Number : ", P.ProcessID," listening ...")
			conn, err := listener.Accept()  //listens for connections
			if err == nil {
				P.IncommingConnections = append(P.IncommingConnections, conn)
				fmt.Println("Listen.Connected with: ", i, conn )
			}
		}
		P.Wg.Done()
	}()
}*/
/*func (P *Process) DialForConnections() {
	var Conn net.Conn
	var err error

	for i := P.ProcessID+1  ; i < P.NumOfProcesses; i++ {
		for {
			if P.ProcessID == 1{
				time.Sleep(6000 * time.Millisecond) // wait for the node 0 to dial to node 2 first
			}
			fmt.Println("Dialing to procces: ", i)
			Conn, err = net.Dial("tcp", Address[i])
			if err == nil {
				break
			}
		}

		fmt.Println("Dial.Connected with :", i, Conn)
		P.OutgoingConnections = append(P.OutgoingConnections, Conn)

	}
}*/
/*func (P *Process) MergeConnections() ([]net.Conn, map[int]net.Conn) {
	var j int = 0
	for i := 0; i < len(P.IncommingConnections); i++ {
		P.Connections = append(P.Connections, P.IncommingConnections[i])
		P.Conns[i] = P.IncommingConnections[i]
		j++
	}
	for i := 0; i < len(P.OutgoingConnections); i++ {
		P.Connections = append(P.Connections, P.OutgoingConnections[i])
		P.Conns[j] = P.OutgoingConnections[i]
		j++
	}
	return P.Connections, P.Conns
}*/

func (P *Process) ListenForConnections() {
	wg.Add(1)
	// Incomming Connections Handled In this Go Routine
	go func() {
		listener, err := net.Listen("tcp", Address[P.ProcessID])   //(1200,1201,1202)
		CheckError(err)
		tempConnsArray := make([]net.Conn, P.NumOfProcesses)//?
		var AceptErr error
		for i := 0; i < P.NumOfProcesses; i++ {
			fmt.Println("Port Number : ", P.ProcessID," listening ...")
			tempConnsArray[i], AceptErr = listener.Accept()  //listens for connections
			CheckError(AceptErr)
			remoteProccId_str, _:= bufio.NewReader(tempConnsArray[i]).ReadString('\n')
			remoteProccId_str = strings.TrimRight(remoteProccId_str, "\n")
			remoteProccId, _:= strconv.Atoi(remoteProccId_str)
			if AceptErr == nil {
				P.ListenConnections[remoteProccId] = tempConnsArray[i]
				fmt.Println("Listen.Connected with remote id :", remoteProccId)
			}
		}
		wg.Done()
	}()
}
func (P *Process) DialForConnections() {
	var Conn net.Conn
	var err error
	for i := 0  ; i < P.NumOfProcesses; i++ {
		for {
			fmt.Println("Dialing to procces: ", i)
			Conn, err = net.Dial("tcp", Address[i])
			if err == nil {
				P.DialConnenctions[i] =  Conn
				fmt.Fprintf(P.DialConnenctions[i], "%d\n", P.ProcessID)
				break
			}
		}
		fmt.Println("Dialed ", i, " and sent my id ", P.ProcessID)

	}
}


func (P *Process) SendMessageToAll(M Msg) {
	for i := 0; i < len(P.Connections); i++ {
		err1 := gob.NewEncoder(P.Connections[i]).Encode(&M)
		if err1 != nil {
			fmt.Println(err1.Error())
		} else {
			fmt.Println("Sent Message")
		}
	}
}

func (P *Process) SendMessageToId(M Msg, id int) {      // sends a messege to a connection specified by the id
	for _, key := range P.AllProcessIDs{
		if id == key {
			err1 := gob.NewEncoder(P.DialConnenctions[key]).Encode(&M)
			if err1 != nil {
				fmt.Println(err1.Error())
			}
		}
	}
}

func (P *Process) ReceiveMessagesSendReply(hbResponse chan detector.Heartbeat ) {
	for _, key := range P.AllProcessIDs{
		P.Wg.Add(1)
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

				if newMsg.MsgType == 0 {
					var heartbeat detector.Heartbeat
					buff:=bytes.NewBuffer(newMsg.Msg)
					err = gob.NewDecoder(buff).Decode(&heartbeat)
					CheckError(err)
					//fmt.Println("Received :",heartbeat )
					if heartbeat.Request == false {
						fmt.Println("Received :",heartbeat )
					}
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
							//fmt.Println("Sent hb Reply")

							MsgReply.MsgType = 0
							MsgReply.Msg = network.Bytes()
							err1 := gob.NewEncoder(P.DialConnenctions[key]).Encode(&MsgReply)
							if err1 != nil {
								fmt.Println(err1.Error())
							}
						}
					}
				}
			}
			P.Wg.Done()
		}(key)
	}
}



func (P *Process) SendHeartbeats(){

		// create a slice of heartbeats to send to the other connected processes

		for _, key := range P.AllProcessIDs{ //creating heartbeat for each connection
			heartbeat := detector.Heartbeat{To: key , From: P.ProcessID, Request: true}
			fmt.Println("Sent :" , heartbeat)
			var network bytes.Buffer
			message := Msg{ Msg: make([]byte,2000),}

			err := gob.NewEncoder(&network).Encode(heartbeat)
			CheckError(err)


			message.Msg = network.Bytes()
			message.MsgType = 0   // type to verify connection
			message.Pid = P.ProcessID

			P.SendMessageToId(message, key)
			time.Sleep(time.Millisecond * 100)
		}
		time.Sleep(time.Millisecond * 1000)

}

//wait for some write in the subscriber channel in order to print out the new leader
func (P *Process) UpdateSubscribe() {
	for {
			time.Sleep(time.Millisecond * 500)
			select {
			case gotLeader := <-P.Subscriber:
				P.ProcessID = gotLeader
				// Got publication, change the leader
				fmt.Println("------ NEW LEADER: ", gotLeader, "------")
			}
	}
}


func main(){
	Proc:= NewProcess()
	Proc.ListenForConnections()
	Proc.DialForConnections()
	wg.Wait()

	fmt.Println("------CONNECTIONS ESTABLISHED------")


	// add the leader Detector Struct
	Proc.LeaderDetectorStruct = detector.NewMonLeaderDetector(Proc.AllProcessIDs)
	Proc.Leader = Proc.LeaderDetectorStruct.Leader()
	fmt.Println("------LEADER IS PROCESS: ", Proc.Leader,"------")

	//subscribe for leader changes
	Proc.Subscriber = Proc.LeaderDetectorStruct.Subscribe()
	go Proc.UpdateSubscribe()    // wait to update the subscriber

	// Add the eventual failure detector
	hbResponse := make(chan detector.Heartbeat, 8) //channel to communicate with fd
	Proc.FailureDetectorStruct = detector.NewEvtFailureDetector(Proc.ProcessID, Proc.AllProcessIDs, Proc.LeaderDetectorStruct, delta, hbResponse)
	Proc.FailureDetectorStruct.Start()

	wg.Add(4)
	Proc.ReceiveMessagesSendReply(hbResponse)

	for {
		Proc.SendHeartbeats()
		time.Sleep(time.Millisecond * 1000)
	}

	wg.Wait()

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
	delete(P.DialConnenctions, index)

	pop := append(P.AllProcessIDs[:index], P.AllProcessIDs[index+1:]...)
	P.AllProcessIDs = pop

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
