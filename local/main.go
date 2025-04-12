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

func getHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "This is a GET request response")
}

func postHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "This is a POST request response")
}

func putHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "This is a PUT request response")
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "This is a DELETE request response")
}

func main() {
    // Create a new ServeMux
    mux := http.NewServeMux()

    // Register multiple handlers with logging middleware
    mux.HandleFunc("/", loggingMiddleware(handler))
    mux.HandleFunc("/get", loggingMiddleware(getHandler))
    mux.HandleFunc("/post", loggingMiddleware(postHandler))
    mux.HandleFunc("/put", loggingMiddleware(putHandler))
    mux.HandleFunc("/delete", loggingMiddleware(deleteHandler))

    // Start the server
    fmt.Println("Server starting on :3000...")
    fmt.Println("Available routes:")
    fmt.Println("- GET    /")
    fmt.Println("- GET    /get")
    fmt.Println("- POST   /post")
    fmt.Println("- PUT    /put")
    fmt.Println("- DELETE /delete")

    if err := http.ListenAndServe(":3000", mux); err != nil {
        log.Fatal(err)
    }
}