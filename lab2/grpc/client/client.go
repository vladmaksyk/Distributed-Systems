// +build !solution

// Leave an empty line above this comment.
package main

// import (
// 	"log"
// 	"os"
// 	"time"
//
// 	"golang.org/x/net/context"
//    "google.golang.org/grpc"
//
// 	pb "github.com/uis-dat520-s2019/vladmaksyk-labs/lab2/grpc/proto"
// )
//
// const (
// 	address     = "localhost:12111"
// 	defaultName = "keyvalue"
// )

func main() {

	// // Set up a connection to the server.
	// conn, err := grpc.Dial(address, grpc.WithInsecure())
	// if err != nil {
	// 	log.Fatalf("did not connect: %v", err)
	// }
	// defer conn.Close()
	// c := pb.NewKeyValueServiceClient(conn)
	//
	// // Contact the server and print out its response.
	// name1 := defaultName
	//
	// if len(os.Args) > 1 {
	// 	name1 = os.Args[1]
	//
	// }
	//
	// ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	// defer cancel()
	//
	// r, err := c.Insert(ctx, &pb.InsertRequest{Key: name1})
	// if err != nil {
	// 	log.Fatalf("could not insert: %v", err)
	// }
	// log.Printf("Insertion: %s", r.Success)

}
