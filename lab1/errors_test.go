package lab1

import (
	. "io"
	"testing"
)

// String: "io: read/write on closed pipe"
var s = ErrClosedPipe.Error()

var errTests = []struct {
	in   Errors
	want string
}{
	{nil, "(0 errors)"},                                                                 //0
	{[]error{}, "(0 errors)"},                                                           //1
	{[]error{ErrClosedPipe}, s},                                                         //2
	{[]error{ErrClosedPipe, ErrClosedPipe}, s + " (and 1 other error)"},                 //3
	{[]error{ErrClosedPipe, ErrClosedPipe, ErrClosedPipe}, s + " (and 2 other errors)"}, //4
	{[]error{nil}, "(0 errors)"},                                                        //5
	{[]error{ErrClosedPipe, nil}, s},                                                    //6
	{[]error{nil, ErrClosedPipe}, s},                                                    //7

	{[]error{ErrClosedPipe, ErrClosedPipe, nil}, s + " (and 1 other error)"},                                //8
	{[]error{ErrClosedPipe, ErrClosedPipe, nil, nil}, s + " (and 1 other error)"},                           //9
	{[]error{ErrClosedPipe, ErrClosedPipe, nil, nil, nil}, s + " (and 1 other error)"},                      //10
	{[]error{nil, ErrClosedPipe, nil, ErrClosedPipe}, s + " (and 1 other error)"},                           //11
	{[]error{nil, nil, ErrClosedPipe, ErrClosedPipe}, s + " (and 1 other error)"},                           //12
	{[]error{ErrClosedPipe, nil, nil, ErrClosedPipe, ErrClosedPipe}, s + " (and 2 other errors)"},           //13
	{[]error{ErrClosedPipe, ErrClosedPipe, nil, ErrClosedPipe, nil}, s + " (and 2 other errors)"},           //14
	{[]error{nil, nil, nil, nil, ErrClosedPipe, ErrClosedPipe, ErrClosedPipe}, s + " (and 2 other errors)"}, //15
}

func TestErrors(t *testing.T) {
	for i, ft := range errTests {
		out := ft.in.Error()
		if out != ft.want {
			t.Errorf("Errors test %d: got %q for input %v, want %q", i, out, ft.in, ft.want)
		}
	}
}
