package main

import (
	"fmt"
	"net/http"
	"os"
	"io"
)

func main() {
	http.Handle("/foo", http.HandlerFunc(fooHandler))
	wrapped := http.HandlerFunc(indexHandler)
	http.Handle("/", LoggingMiddleware(wrapped))
	fmt.Println("Server Starting! localhost:8080/")
	http.ListenAndServe("0.0.0.0:8080", nil)
}

func fooHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "no-cache")
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte("Foo"))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "no-cache")
	w.Header().Add("Content-Type", "text/html; charset=utf-8")

	file, err := os.Open("./index.html")
	if err != nil {
		http.Error(w, "Something went wrong!", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
    if err != nil {
        http.Error(w, "Something went wrong!", http.StatusInternalServerError)
		return
    }

	w.Write(data)
}
