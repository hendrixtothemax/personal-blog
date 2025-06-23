package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.Handle("/foo", http.HandlerFunc(fooHandler))
	http.Handle("/", http.HandlerFunc(indexHandler))
	fmt.Println("Server Starting! localhost:8080/")
	http.ListenAndServe(":8080", nil)
}

func fooHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "no-cache")
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte("Foo"))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "no-cache")
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`
	<!DOCTYPE html>
	<head>
		<title>Go Server</title>
	</head>
	<body>
		<h1>Hello World!</h1>
	</body>
	`))
}
