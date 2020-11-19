// +build !solution

package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
)

var httpAddr = flag.String("http", ":8080", "Listen address")

func main() {
	flag.Parse()
	server := NewServer()
	log.Fatal(http.ListenAndServe(*httpAddr, server))
}

// Server implements the web server specification found at
// https://github.com/uis-dat520-s2019/assignments/blob/master/lab2/README.md#web-server
type Server struct {
	// TODO(student): Add needed fields
	count int
}

// NewServer returns a new Server with all required internal state initialized.
// NOTE: It should NOT start to listen on an HTTP endpoint.
func NewServer() *Server {
	s := &Server{
		count: 0,
	}
	// TODO(student): Implement
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO(student): Implement
	s.count++

	if r.URL.Path[0:] == "/" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello World!\n"))
	} else if r.URL.Path[0:] == "/lab2" {
		w.WriteHeader(http.StatusMovedPermanently)
		w.Write([]byte("<a href=\"http://www.github.com/uis-dat520-s2019/assignments/tree/master/lab2\">Moved Permanently</a>.\n\n"))
	} else if r.URL.Path[0:] == "/counter" {
		w.WriteHeader(http.StatusOK)
		Response := ("counter: " + strconv.Itoa(s.count) + "\n")
		w.Write([]byte(Response))
	} else if r.URL.Path[0:] == "/fizzbuzz" {
		V := r.URL.Query().Get("value")
		Value, err := strconv.Atoi(V)

		if err == nil {
			if Value%3 == 0 && Value%5 != 0 {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("fizz\n"))
			} else if Value%3 != 0 && Value%5 == 0 {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("buzz\n"))
			} else if Value%3 == 0 && Value%5 == 0 {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("fizzbuzz\n"))
			} else {
				Str := strconv.Itoa(Value)
				Response := Str + "\n"
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(Response))
			}
		} else {
			if len(V) == 0 { // comes here if there is no value
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("no value provided\n"))

			} else { // comes here if value is not an integer
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("not an integer\n"))
				//fmt.Println("not a number")
			}
		}

	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 page not found\n"))
	}

}
