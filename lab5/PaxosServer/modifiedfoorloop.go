package main

import "fmt"

var alowance bool

func main(){
   alowance = true

      for i := 0; i < 10; {
         if alowance {
            fmt.Println("Inside")
            i++
            alowance = false
         }else{
            fmt.Println("Outside")
            i++
         }
      }


}
