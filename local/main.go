package main

import (
	"fmt"
	"log"
	"net/http"
)

func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Log request details
        fmt.Printf("Method: %s | Path: %s | Remote Addr: %s\n", r.Method, r.URL.Path, r.RemoteAddr)
        next(w, r)
    }
}

func handler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hello from the server!")
}

func main() {
    // Create a new ServeMux
    mux := http.NewServeMux()

    // Register the handler with logging middleware
    mux.HandleFunc("/", loggingMiddleware(handler))

    // Start the server
    fmt.Println("Server starting on :3000...")
    if err := http.ListenAndServe(":4000", mux); err != nil {
        log.Fatal(err)
    }
}