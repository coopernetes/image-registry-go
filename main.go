package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
)

const (
	// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pulling-manifests
	nameRegex string = "[a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*"
	refRegex  string = "[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}"
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
	rootDir := setupStorage()
	log.Printf("Storage: %s", rootDir)
	http.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		printInfo(r)
		name, err := parseName(r.RequestURI)
		if err != nil {
			es := fmt.Sprintf("Unexpected error encountered: %s", err.Error())
			_, err := w.Write([]byte(es))
			if err != nil {
				log.Printf("ERROR! Could not write error in request?! %s", err.Error())
			}
			w.WriteHeader(500)
		}
		if !validName(name) {
			w.Write(writeError("NAME_INVALID", "invalid repository name"))
			w.WriteHeader(400)
		}
		log.Printf(name)
		endpoint := strings.TrimPrefix(r.RequestURI, strings.Join([]string{"/v2/", name}, ""))
		if strings.Contains(endpoint, "/blobs/uploads/") {
			handleUpload(name, r, rootDir)
		}
		w.WriteHeader(200)
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func setupStorage() string {
	dir, wdErr := os.Getwd()
	if wdErr != nil {
		log.Fatalf(wdErr.Error())
	}
	dir = path.Join(dir, "images")
	_, readErr := os.ReadDir(dir)
	if readErr != nil {
		if errors.Is(readErr, fs.ErrNotExist) {
			mkErr := os.MkdirAll("images", 0755)
			if mkErr != nil {
				log.Fatalf(mkErr.Error())
			}
		}
		if !errors.Is(readErr, fs.ErrExist) {
			log.Fatalf(readErr.Error())
		}
	}
	return dir
}

func printInfo(r *http.Request) {
	client := r.Host
	method := r.Method
	uri := r.RequestURI
	conType := r.Header.Get("Content-Type")

	log.Printf("Request details:")
	log.Printf("\tHost: %s", client)
	log.Printf("\tMethod: %s", method)
	log.Printf("\tURI: %s", uri)
	log.Printf("\tHost: %s", conType)
}

func writeError(code string, message string) []byte {
	e := ErrorResponse{
		Errors: []ErrorDetail{
			ErrorDetail{
				Code:    code,
				Message: message,
				Detail:  "{}",
			},
		},
	}
	out, err := json.Marshal(e)
	if err != nil {
		log.Printf("Unable to marshall error response: %s", err.Error())
	}
	return out
}

func parseName(url string) (string, error) {
	s := strings.TrimPrefix(url, "/v2/")
	paths := strings.Count(s, "/")
	var name = ""
	if paths <= 1 {
		return "", errors.New(fmt.Sprintf("URL does not match any valid OCI endpoint: %s", url))
	}
	if paths == 2 {
		name = strings.Split(s, "/")[0]
	} else {
		parts := make([]string, 0)
		for _, p := range strings.Split(s, "/") {
			if p == "blobs" || p == "manifests" || p == "tags" || p == "referrers" {
				break
			}
			parts = append(parts, p)
		}
		name = strings.Join(parts, "/")
	}
	return name, nil
}

func validName(name string) bool {
	matched, err := regexp.MatchString(nameRegex, name)
	if err != nil {
		log.Printf("Error while parsing regex: %s", err.Error())
	}
	return matched
}

func handleUpload(name string, r *http.Request, rootDir string) error {
	return nil
}
