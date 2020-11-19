package main

import ("time"
   "sync"
   )

var TimeoutSignal *time.Ticker
var wg = sync.WaitGroup{}

func main(){
   wg.Add(2)

   TimeoutSignal = time.NewTicker(time.Second * 3)
   time.Sleep(time.Second * 2)
   TimeoutSignal = time.NewTicker(time.Second * 3)

   go listen(TimeoutSignal)
   TimeoutSignal = time.NewTicker(time.Second * 10)



   wg.Wait()
}

func listen(TimeoutSignal *time.Ticker){
   for{
      select{
		case <- TimeoutSignal.C:
         print("Timeout")
         wg.Done()
      default:
         continue
      }
   }
}
