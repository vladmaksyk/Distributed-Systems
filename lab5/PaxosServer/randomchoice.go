package main

import (
  "math/rand"
  "time"
  "fmt"
)



func RandomFromSlice(slice []int) int {
    rand.Seed(time.Now().Unix())
    message := slice[rand.Intn(len(slice))]
    return message
}

func main() {

reasons := make([]int, 0)
reasons = append(reasons, 0,1,2)

result := RandomFromSlice(reasons)
fmt.Println(result)




}
