package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pulling-manifests
	imageRegex string = "[a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*"
)

type ErrorResponse struct {
	Errors []ErrorDetail `json:"errors"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

func main() {
	fmt.Println("Starting...")
	http.HandleFunc("/v2/", v2Handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func parseImage(url string) (string, string, error) {
	if !strings.HasPrefix(url, "/v2/") {
		return "", "", errors.New("Not a valid url")
	}
	trimmed := strings.TrimPrefix(url, "/v2/")
	parts := strings.Split(trimmed, "/manifests/")
	return parts[0], parts[1], nil
}

func v2Handler(w http.ResponseWriter, r *http.Request) {
	log.Printf("URL: %s", r.RequestURI)
	log.Printf("Content-Type: %s", r.Header.Get("Content-Type"))
	log.Printf("Accept: %s", r.Header.Get("Accept"))
	if r.RequestURI == "/v2/" {
		w.WriteHeader(200)
	}
	h_accept := r.Header.Get("Accept")
	switch h_accept {
	case specs.MediaTypeImageIndex:
		log.Print("OCI index media type hit")
		w.Header().Set("Content-Type", h_accept)
	case "application/vnd.docker.distribution.manifest.v1+prettyjws":
		log.Print("Docker v1+prettyjws media type hit")
		w.Header().Set("Content-Type", h_accept)
	case "application/vnd.docker.distribution.manifest.v2+json":
		log.Print("Docker v2+json media type hit")
		w.Header().Set("Content-Type", h_accept)
	case "application/json":
		log.Print("JSON media type hit")
		w.Header().Set("Content-Type", h_accept)
	default:
		log.Printf("Unexpected or empty Accept header: %s", h_accept)
	}
	if r.Method == "HEAD" && strings.Contains(r.RequestURI, "/manifests/") {
		existHandler(w, r)
	}
	if r.Method == "GET" && strings.Contains(r.RequestURI, "/manifests/") {
		pullHandler(w, r)
	}
}

func existHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Hit exist handler, uri: %s", r.RequestURI)
	m := make(map[string][]string)
	m["test/image"] = []string{"latest"}
	m["anotherimage"] = []string{"latest", "v0.0.1", "v0.0.2"}
	req_i, req_ref, err := parseImage(r.RequestURI)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%s:%s", req_i, req_ref)
	for image, refs := range m {
		for _, ref := range refs {
			if r.RequestURI == fmt.Sprintf("/v2/%s/manifests/%v", image, ref) {
				log.Printf("Matched image %s:%s", image, ref)
				w.WriteHeader(200)
			}
		}
	}
	log.Printf("No matching manifests found for %s", r.RequestURI)
	errorHandler(w, r, ErrorDetail{
		Code:    "BLOB_UNKNOWN",
		Message: fmt.Sprintf("Image %s:%s not found", req_i, req_ref),
		Detail:  fmt.Sprintf("uri=%s", r.RequestURI),
	}, 404)
}

func pullHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Hit pull handler, uri: %s", r.RequestURI)
	m := make(map[string][]string)
	m["test/image"] = []string{"latest"}
	m["anotherimage"] = []string{"latest", "v0.0.1", "v0.0.2"}
	req_i, req_ref, err := parseImage(r.RequestURI)
	if err != nil {
		log.Fatal(err)
	}
	for image, refs := range m {
		for _, ref := range refs {
			if r.RequestURI == fmt.Sprintf("/v2/%s/manifests/%v", image, ref) {
				log.Printf("Matched image %s:%s", image, ref)
				w.WriteHeader(200)
				return
			}
		}
	}
	log.Printf("No matching manifests found for %s", r.RequestURI)
	errorHandler(w, r, ErrorDetail{
		Code:    "BLOB_UNKNOWN",
		Message: fmt.Sprintf("Image %s:%s not found", req_i, req_ref),
		Detail:  fmt.Sprintf("uri=%s", r.RequestURI),
	}, 404)
}

func pushHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Hit %s", r.RequestURI)
	rc, err := r.GetBody()
	if err != nil {
		fmt.Fprintf(w, "Something went wrong!")
	}
	data := make([]byte, r.ContentLength)
	rc.Read(data)
	var idx specs.Index
	json.Unmarshal(data, &idx)
	log.Print(idx)
}

func errorHandler(w http.ResponseWriter, r *http.Request, errDetail ErrorDetail, statusCode int) {
	resp := ErrorResponse{
		Errors: []ErrorDetail{errDetail},
	}
	log.Print(resp)
	d, err := json.Marshal(resp)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%s", string(d[:]))
	w.WriteHeader(statusCode)
	w.Write(d)
}
