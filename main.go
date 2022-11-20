package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/distribution/distribution/uuid"
	_ "github.com/opencontainers/image-spec/specs-go/v1"
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
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	rootDir := setupStorage()
	log.Printf("Storage: %s", rootDir)
	http.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		printInfo(r)
		name, err := parseName(r.RequestURI)
		if err != nil {
			writeServerError(err, w)
		}
		if !validName(name) {
			w.WriteHeader(400)
			_, err := w.Write(writeError("NAME_INVALID", "invalid repository name"))
			if err != nil {
				writeServerError(err, w)
			}
		}
		log.Printf(name)
		endpoint := strings.TrimPrefix(r.RequestURI, strings.Join([]string{"/v2/", name}, ""))
		if r.Method == "POST" && strings.HasSuffix(endpoint, "/blobs/uploads/") {
			id := uuid.Generate().String()
			w.Header().Set("Location", r.RequestURI+id)
			w.WriteHeader(202)
		}
		if r.Method == "PUT" && strings.Contains(endpoint, "/blobs/uploads/") {
			err := os.MkdirAll(path.Join(rootDir, name, "_uploads"), 0755)
			if err != nil {
				writeServerError(err, w)
			}
			digest := r.FormValue("digest")
			log.Printf("Digest: %s", digest)
			destFile := path.Join(rootDir, name, "_uploads", digest)
			var f *os.File
			if _, statE := os.Stat(destFile); os.IsNotExist(statE) {
				innerF, err := os.OpenFile(destFile, os.O_RDWR|os.O_CREATE, 0644)
				if err != nil {
					writeServerError(err, w)
				}
				f = innerF
			} else {
				innerF, err := os.OpenFile(destFile, os.O_RDWR|os.O_CREATE, 0644)
				if err != nil {
					writeServerError(err, w)
				}
				err = os.Truncate(destFile, 0)
				if err != nil {
					writeServerError(err, w)
				}
				f = innerF
			}
			writeToFile(f, w, r)
		}
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func writeServerError(err error, w http.ResponseWriter) {
	w.WriteHeader(500)
	es := fmt.Sprintf("Unexpected error encountered: %s", err.Error())
	_, writeErr := w.Write([]byte(es))
	if writeErr != nil {
		log.Panicf("ERROR! Could not write error in request?! %s", err.Error())
	}
}

func writeToFile(f *os.File, w http.ResponseWriter, r *http.Request) {
	total := r.ContentLength
	log.Printf("Total %d", total)
	buf := make([]byte, 1024)
	for {
		n, err := r.Body.Read(buf)
		log.Printf("Read %d", n)
		_, err2 := f.Write(buf[0:n])
		if err2 != nil {
			log.Printf("Failed to write buffer to file: %s", err2)
		}
		log.Printf("Wrote %d", n)
		if err == io.EOF {
			log.Print("Reached EOF")
			break
		}
		total = total - int64(n)
		log.Printf("Total to read %d", total)
		if total > 0 {
			for i := 0; i < 1024; i++ {
				buf[i] = 0
			}
		}
	}
	w.WriteHeader(201)
}

func setupStorage() string {
	dir, wdErr := os.Getwd()
	if wdErr != nil {
		log.Printf(wdErr.Error())
	}
	dir = path.Join(dir, "data")
	_, readErr := os.ReadDir(dir)
	if readErr != nil {
		if errors.Is(readErr, fs.ErrNotExist) {
			mkErr := os.MkdirAll(dir, 0755)
			if mkErr != nil {
				log.Printf(mkErr.Error())
			}
		} else {
			log.Printf(readErr.Error())
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
		Errors: []ErrorDetail{{
			Code:    code,
			Message: message,
			Detail:  "{}",
		}},
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

func createSessionId(name string, r *http.Request) error {

	return nil
}
