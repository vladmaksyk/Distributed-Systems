// +build !solution

package lab1

import "fmt"

// Errors is an error returned by a multiwriter when there are errors with one
// or more of the writes. Errors will be in a one-to-one correspondence with
// the input elements; successful elements will have a nil entry.
//
// Should not be constructed if all entires are nil; in this case one should
// instead return just nil instead of a MultiError.

/*
Task 4: Errors needed for multiwriter

You may find this blog post useful:
http://blog.golang.org/error-handling-and-go

Similar to a the Stringer interface, the error interface also defines a
method that returns a string.

type error interface {
    Error() string
}

Thus also the error type can describe itself as a string. The fmt package (and
many others) use this Error() method to print errors.

Implement the Error() method for the Errors type defined above.

For the following conditions should be covered.

1. When there are no errors in the slice, it should return:

"(0 errors)"

2. When there is one error in the slice, it should return:

The error string return by the corresponding Error() method.

3. When there are two errors in the slice, it should return:

The first error + " (and 1 other error)"

4. When there are X>2 errors in the slice, it should return:

The first error + " (and X other errors)"
*/
type Errors []error

func (m Errors) Error() string {
	if len(m) == 0 {
		return "(0 errors)"

	} else if len(m) == 1 {
		if m[0] == nil {
			return "(0 errors)"
		}
		return fmt.Sprintf("%s", m[0])

	} else if len(m) == 2 {
		if m[0] == nil || m[1] == nil {
			i := 0
			for i < len(m) {
				if m[i] != nil {
					return fmt.Sprintf("%s", m[i])
				}
				i += 1
			}
		} else {
			return fmt.Sprintf("%s (and 1 other error)", m[0])
		}

	} else if len(m) > 2 {
		errorscount := 0
		for j := 0; j < len(m); j++ { // counts the amount of errors
			if m[j] != nil {
				errorscount += 1
			}
		}
		k := 0
		for k < len(m) {
			if m[k] != nil {
				if (errorscount - 1) > 1 {
					return fmt.Sprintf("%s (and %d other errors)", m[k], errorscount-1)
				} else {
					return fmt.Sprintf("%s (and %d other error)", m[k], errorscount-1)
				}
			}
			k += 1
		}

	}
	return ""
}
