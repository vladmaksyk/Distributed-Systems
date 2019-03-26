## Answers to Paxos Questions 

You should write down your answers to the
[questions](https://github.com/uis-dat520-s18/labassignments-2015/tree/master/lab4-singlepaxos#questions-10)
for Lab 4 in this file. 

1. 
Yes, it's possible that Paxos enters in an infinite loop if and only if there isn't a leader implementation and each process try to propose a different number, accept the highest value and the other continue to propose greater numbers. 

2. 
No, it's not included because in each `Prepare` message the process send a number N, that is equivalent to the roundID of the process; the value V is the message to send.

3. 
Yes, Paxos rely on the increment of the proposal/round number because otherwise the execution of the algorithm will not stop since the processes will not be able to decide whicht value accept from the set of propose values sent from the processes.

4. 
- “the value it last accepted” is the message sent by the previuos highest-numbered process.
- “instance” is the process that respond with the value, since every process save in their memory the actual value to send.

5. 
If there are multiple proposer, we can be in a situation in which nobody will learn anything because all the process will be stuck in the propose/accept phase, since for example, process p propose n1 and it will be accepted, another process q propose n2 > n1 and it will be accept; so phase 2 of process p will be ignored and it will propose a new number n3 > n2 causing the pahse 2 of process q to be ignored, and so on.

6. 
The process with the higest rank will receive the promise, the other process will be ignored so it will propose a new value, higher than the one accepted right now, and it will be accept; this will create a infinite loop.

7. ---

8. 
If we are in the single paxos, an acceptor can only accept one value.

9. 
In order to choose a value, the proposer must receive a majority of promise from the acceptor, if it's happened the value is chosen from the highest round that propose that value. Basically, chosen values and learned values are the same with the only different that the learned values are sent to the clients instead the chosen values are still inside the proposer/acceptor.
