// +build !solution

package lab1

import (
	"fmt"
	"io"
)

/*
Task 3: Rot 13

This task is taken from http://tour.golang.org.

A common pattern is an io.Reader that wraps another io.Reader, modifying the
stream in some way.

For example, the gzip.NewReader function takes an io.Reader (a stream of
compressed data) and returns a *gzip.Reader that also implements io.Reader (a
stream of the decompressed data).

Implement a rot13Reader that implements io.Reader and reads from an io.Reader,
modifying the stream by applying the rot13 substitution cipher to all
alphabetical characters.

The rot13Reader type is provided for you. Make it an io.Reader by implementing
its Read method.
*/

func rot13func(b byte) byte {
	var first, last byte

	if 'a' <= b && b <= 'z' {
		first, last = 'a', 'z'
	} else if 'A' <= b && b <= 'Z' {
		first, last = 'A', 'Z'
	} else {
		return b
	}

	return (b-first+13)%(last-first+1) + first
}

type rot13Reader struct {
	root io.Reader
}

//test
func (root13 rot13Reader) Read(p []byte) (n int, err error) {

	n, err = root13.root.Read(p)
	for i := 0; i < n; i++ {
		p[i] = rot13func(p[i])
		fmt.Println(p[i])
	}
	return n, err
	//return 0, nil

}
