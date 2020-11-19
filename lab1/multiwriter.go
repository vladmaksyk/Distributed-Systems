// +build !solution

package lab1

import (
	"io"
	//"fmt"
)

/*
Task 5: WriteTo function for multiple writers

In this task you are going to implement a WriteTo function that writes to
multiple writers. This is similar to the io.MultiWriter() function. However,
the io.MultiWriter() function can only return a single error, and it is not
possible to figure out which of the original writers caused the error. That
is, you must use the Errors type that you developed for Task 4.

Implement the WriteTo() function defined below.

For the following conditions should be satisfied.

1. Write the []byte slice to all writers.

2. The function should return (using n) the bytes written by each writer with
index position corresponding to the index position of the writer.([]int{9, 9},)

An empty slice ([]int{}) should be returned if there are no writers as argument ([]io.Writer{})to the
function.

3. If one of the writers returned an error, that error should be returned in
the index position corresponding to the index position of the writer.

4. If one of the writers could not write the entire buffer, the error
io.ErrShortWrite should be returned in the index position corresponding to
that writer's index position.

5. If no errors were observed, the function must return n, nil.
*/

// WriteTo writes b to the provided writers, returns a slice of the number
// of byte written to each writer, and a slice of errors, if any.
func WriteTo(b []byte, writers ...io.Writer) (n []int, errs Errors) {
	//fmt.Println("b :", len(b))
	//fmt.Println("writers :", len(writers))

	Num_bytes := len(b) // buffer lenght for all writers in iteration
	Num_writers := len(writers)

	Bytes_writen_by_each_writer := make([]int, Num_writers)
	Errors := make([]error, Num_writers)

	for i := 0; i < Num_writers; i++ {
		n, err := writers[i].Write(b)
		Bytes_writen_by_each_writer[i] = n
		if n < Num_bytes {
			Errors[i] = io.ErrShortWrite
		} else {
			Errors[i] = err
		}
	}

	for j := 0; j < Num_writers; j++ { // if there were erors return them otherwise if all nil return one nil.
		if Errors[j] != nil {
			return Bytes_writen_by_each_writer, Errors
			break
		}
	}

	return Bytes_writen_by_each_writer, nil
}

//
