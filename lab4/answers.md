## Answers to Paxos Questions 

You should write down your answers to the
[questions](https://dat520.github.io/r/?assignments/tree/master/lab4-singlepaxos#questions-10)
for Lab 4 in this file. 

1. Your answer.
Paxos can enter an infinite loop if there is no leader implementation. 
Each process will try to propose a different value accept the value with 
the highest round and so on.

2. Your answer.
No its not included. The value to agree on is included in the accept message.

3. Your answer.
Yes, Paxos rely on the increment of the proposal round because otherwise the acceptor won't be
able to determine weather the value is new or old.

4. Your answer.
The value it last accepted is the value sent by one of the previous processes with the highest round.


5. Your answer.
In situation when we have multiple proposers we can get in a situation where 
no one will learn anything since everyone will be stuck in the propose/accept phase

6. Your answer.
The process with the highest rank will receive the promise the other process will be ignored so 
it will propose a new value higher than the one accepted right now and it will be accepted.This will create 
an infinite loop.

7. Your answer.
If there are multiple proposers the system can get desynchronized. 

8. Your answer.
In a single paxos acceptor can only accept one value

9. Your answer.
In order to choose a value the proposer must receive the majority of promise 
messages from the acceptor. The connection between chosen values and accept values is that chosen values 
eventually become learned values.

