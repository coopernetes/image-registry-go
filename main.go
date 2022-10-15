package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type ErrorResponse struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message"`
}

func main() {
	fmt.Println("Starting...")
	http.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		rc, err := r.GetBody()
		if err != nil {
			fmt.Fprint(w, &ErrorResponse{
				Code:    500,
				Message: "Something went wrong!",
			})
		}
		data := make([]byte, r.ContentLength)
		rc.Read(data)
		var idx specs.Index
		json.Unmarshal(data, &idx)
		log.Print(idx)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("URL: %s", r.RequestURI)
		log.Printf("Content-Type: %s", r.Header.Get("Content-Type"))
		log.Printf("Accept: %s", r.Header.Get("Accept"))
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
