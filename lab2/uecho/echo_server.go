// +build !solution

// Leave an empty line above this comment.
package main

import (
	"fmt"
	"net"
	"strings"
	"unicode"
)

// UDPServer implements the UDP server specification found at
// https://github.com/uis-dat520-s2019/assignments/blob/master/lab2/README.md#udp-server
type UDPServer struct {
	conn *net.UDPConn
}

// NewUDPServer returns a new UDPServer listening on addr. It should return an
// error if there was any problem resolving or listening on the provided addr.
func NewUDPServer(addr string) (*UDPServer, error) {
	//fmt.Println("level44")
	var ServerConn UDPServer
	var err, err1 error

	// Lets prepare a address at any address at port addr
	//func ResolveUDPAddr(network, address string) (*UDPAddr, error)
	ServerAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	// Now listen at selected port
	//func ListenUDP(network string, laddr *UDPAddr) (*UDPConn, error)
	ServerConn.conn, err1 = net.ListenUDP("udp", ServerAddr)
	if err1 != nil {
		return nil, err1
	} else {
		return &ServerConn, nil
	}
}

// ServeUDP starts the UDP server's read loop. The server should read from its
// listening socket and handle incoming client requests as according to the
// the specification.
func (u *UDPServer) ServeUDP() {
	//fmt.Println("im in ServeUDP")
	MessageBuff := make([]byte, 1024)
	Temp := make([]string, 2)
	var message, cmd, text, returnString string
	var ValidCommand = false

	for {
		//func (c *UDPConn) ReadFromUDP(b []byte) (int, *UDPAddr, error)
		n, UDPAddres, err := u.conn.ReadFromUDP(MessageBuff) //n is len of the message, buff is the recieved message, UDPAdress is the Udp adress
		if err != nil {
			fmt.Println("Error: ", err)
		}

		message = string(MessageBuff[0:n])
		Temp = strings.Split(message, "|:|")
		cmd = Temp[0]
		text = Temp[1]

		Commands := [5]string{"UPPER", "LOWER", "ROT13", "SWAP", "CAMEL"}
		for _, str := range Commands {
			if text != "" && str == cmd {
				ValidCommand = true
				break
			} else {
				ValidCommand = false
			}
		}

		if ValidCommand {
			switch cmd {
			case "UPPER":
				returnString = strings.ToUpper(text)
			case "LOWER":
				returnString = strings.ToLower(text)
			case "ROT13":
				returnString = Rot13(text)
			case "SWAP":
				returnString = Swap(text)
			case "CAMEL":
				returnString = Camel(text)
			}
		} else {
			returnString = "Unknown command"
		}

		//func (c *UDPConn) WriteToUDP(b []byte, addr *UDPAddr) (int, error)
		_, err1 := u.conn.WriteToUDP([]byte(returnString), UDPAddres)
		if err1 != nil {
			fmt.Println("Error: ", err1)
		}

	}
}

func Rot13(text string) string {
	var first, last byte
	buffer := []byte(text)
	ByteString := make([]byte, 1024)
	LenString := len(buffer)

	for i := 0; i < LenString; i++ {
		if 'a' <= buffer[i] && buffer[i] <= 'z' {
			first, last = 'a', 'z'
		} else if 'A' <= buffer[i] && buffer[i] <= 'Z' {
			first, last = 'A', 'Z'
		} else {
			ByteString[i] = buffer[i]
			continue
		}
		ByteString[i] = (buffer[i]-first+13)%(last-first+1) + first
	}
	returnString := string(ByteString[0:LenString])
	return returnString
}

func Swap(text string) string {
	buffer := []rune(text)
	returnString := make([]rune, 100)
	LenString := len(buffer)

	for i := 0; i < LenString; i++ {
		if unicode.IsUpper(buffer[i]) {
			returnString[i] = unicode.ToLower(buffer[i])
		} else {
			returnString[i] = unicode.ToUpper(buffer[i])
		}
	}

	returnStringFinal := string(returnString[0:LenString])
	return returnStringFinal

}

func Camel(s string) string {
	ReturnString := ""
	capNext := true
	for _, v := range s {
		if v >= 'A' && v <= 'Z' {
			if capNext {
				ReturnString += string(v)
			} else {
				ReturnString += strings.ToLower(string(v))
			}
		}
		if v >= 'a' && v <= 'z' {
			if capNext {
				ReturnString += strings.ToUpper(string(v))
			} else {
				ReturnString += string(v)
			}
		}
		if v == ' ' { // decides if the next letter has to be capita
			ReturnString += string(v)
			capNext = true
		} else {
			capNext = false
		}
	}
	return ReturnString
}

// socketIsClosed is a helper method to check if a listening socket has been
// closed.
func socketIsClosed(err error) bool {
	if strings.Contains(err.Error(), "use of closed network connection") {
		return true
	}
	return false
}
