package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/uis-dat520-s18/glabs/grouplab2/singlepaxos"
)

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
