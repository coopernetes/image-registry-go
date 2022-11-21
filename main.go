package main

import (
	"bytes"
	"crypto/sha256"
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
	nameRegex   string = "^[a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*$"
	refRegex    string = "^[a-zA-Z0-9_][a-zA-Z0-9._-]{1,127}$"
	digestRegex string = "^sha256:([a-f0-9]{64})$"
)

type ErrorResponse struct {
	Errors []ErrorDetail `json:"errors"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

type TagList struct {
	Name    string   `json:"name"`
	TagList []string `json:"tags"`
}

func main() {
	fmt.Println("Starting...")
	logFlags := log.LstdFlags | log.LUTC
	if e := os.Getenv("DEBUG"); e != "" {
		logFlags = logFlags | log.Lshortfile
	}
	log.SetFlags(logFlags)
	rootDir := setupStorage()
	log.Printf("Storage: %s", rootDir)
	http.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		if e := os.Getenv("DEBUG"); e != "" {
			printInfo(r)
		}
		name, err := parseName(r.RequestURI)
		if err != nil {
			writeServerError(err, w)
			return
		}
		if !matches(nameRegex, name) {
			writeOciError("NAME_INVALID", "invalid repository name", w, 400)
			return
		}
		endpoint := strings.TrimPrefix(r.RequestURI, strings.Join([]string{"/v2/", name}, ""))
		if r.Method == "HEAD" && strings.Contains(endpoint, "/blobs/sha256:") {
			parts := strings.Split(endpoint, "/")
			requestDigest := parts[len(parts)-1]
			if !matches(digestRegex, requestDigest) {
				writeOciError("BLOB_UNKNOWN", "blob unknown to registry", w, 400)
				return
			}
			b, err := fileExists(path.Join(rootDir, name, "_blobs", requestDigest))
			var status int
			if err != nil {
				writeServerError(err, w)
				return
			}
			if b {
				w.Header().Set("Docker-Content-Digest", requestDigest)
				status = 200
			} else {
				status = 404
			}
			w.WriteHeader(status)
		}
		if r.Method == "GET" && strings.Contains(endpoint, "/blobs/sha256:") {
			parts := strings.Split(endpoint, "/")
			requestDigest := parts[len(parts)-1]
			blobPath := path.Join(rootDir, name, "_blobs", requestDigest)
			b, err := fileExists(blobPath)
			var status int
			if err != nil {
				writeServerError(err, w)
				return
			}
			if b {
				w.Header().Set("Docker-Content-Digest", requestDigest)
				status = 200
				content, e := readFile(blobPath)
				if e != nil {
					writeServerError(e, w)
					return
				}
				_, err := content.WriteTo(w)
				if err != nil {
					writeServerError(err, w)
					return
				}
			} else {
				status = 404
				w.WriteHeader(status)
			}
		}
		if r.Method == "POST" && strings.HasSuffix(endpoint, "/blobs/uploads/") {
			id := uuid.Generate().String()
			w.Header().Set("Location", r.RequestURI+id)
			w.WriteHeader(202)
		}
		if r.Method == "PUT" && strings.Contains(endpoint, "/blobs/uploads/?") {
			err := os.MkdirAll(path.Join(rootDir, name, "_blobs"), 0755)
			if err != nil {
				writeServerError(err, w)
				return
			}
			digest := r.FormValue("digest")
			log.Printf("Digest: %s", digest)
			destFile := path.Join(rootDir, name, "_blobs", digest)
			writeToFile(destFile, w, r)
		}
		if r.Method == "GET" && strings.HasSuffix(endpoint, "/tags/list") {
			if _, err := os.ReadDir(path.Join(rootDir, name)); err != nil {
				writeOciError("NAME_UNKNOWN", "repository name not known to registry", w, 404)
				return
			}
			tags, err := getTags(path.Join(rootDir, name))
			if err != nil {
				writeServerError(err, w)
				return
			}
			tl := TagList{
				Name:    name,
				TagList: tags,
			}
			jb, jE := json.Marshal(tl)
			if jE != nil {
				writeServerError(jE, w)
				return
			}
			_, wE := w.Write(jb)
			if wE != nil {
				writeServerError(wE, w)
				return
			}
		}
		if r.Method == "PUT" && strings.Contains(endpoint, "/manifests/") {
			parts := strings.Split(endpoint, "/manifests/")
			requestRef := parts[len(parts)-1]
			if !matches(refRegex, requestRef) {
				writeOciError("MANIFEST_INVALID", "manifest invalid", w, 400)
				return
			}
			err := os.MkdirAll(path.Join(rootDir, name, requestRef), 0755)
			if err != nil {
				writeServerError(err, w)
				return
			}
			destFile := path.Join(rootDir, name, requestRef, "manifest.json")
			writeToFile(destFile, w, r)
		}
		if r.Method == "HEAD" && strings.Contains(endpoint, "/manifests/") {
			parts := strings.Split(endpoint, "/")
			lastPart := parts[len(parts)-1]
			isRef := matches(refRegex, lastPart)
			isDigest := matches(digestRegex, lastPart)

			if !(isRef || isDigest) {
				writeOciError("MANIFEST_INVALID", "manifest invalid", w, 404)
				return
			}
			manifestPath := path.Join(rootDir, name)
			if isRef {
				manifestPath = path.Join(manifestPath, lastPart, "manifest.json")
			} else {
				foundPath, err := findManifest(rootDir, name, lastPart)
				if err != nil {
					return
				}
				if foundPath == "" {
					writeOciError("MANIFEST_UNKNOWN", "manifest unknown to registry", w, 404)
					return
				}
				manifestPath = foundPath
			}
			log.Printf("Manifest path: %s", manifestPath)
			b, err := fileExists(manifestPath)
			var status int
			if err != nil {
				writeServerError(err, w)
				return
			}
			if b {
				status = 200
			} else {
				status = 404
			}
			w.WriteHeader(status)
		}
		if r.Method == "GET" && strings.Contains(endpoint, "/manifests/") {
			parts := strings.Split(endpoint, "/")
			lastPart := parts[len(parts)-1]
			isRef := matches(refRegex, lastPart)
			isDigest := matches(digestRegex, lastPart)

			if !(isRef || isDigest) {
				writeOciError("MANIFEST_INVALID", "manifest invalid", w, 404)
				return
			}
			manifestPath := path.Join(rootDir, name)
			if isRef {
				manifestPath = path.Join(manifestPath, lastPart, "manifest.json")
			} else {
				foundPath, err := findManifest(rootDir, name, lastPart)
				if err != nil {
					return
				}
				if foundPath == "" {
					writeOciError("MANIFEST_UNKNOWN", "manifest unknown to registry", w, 404)
					return
				}
				manifestPath = foundPath
			}
			b, err := fileExists(manifestPath)
			if err != nil {
				writeServerError(err, w)
				return
			}
			if b {
				content, e := readFile(manifestPath)
				if e != nil {
					writeServerError(e, w)
					return
				}
				_, err := content.WriteTo(w)
				if err != nil {
					writeServerError(err, w)
					return
				}
			} else {
				writeOciError("MANIFEST_UNKNOWN", "manifest unknown to registry", w, 404)
				return
			}
		}
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func getTags(path string) ([]string, error) {
	tags := make([]string, 0)
	files, err := os.ReadDir(path)
	if err != nil {
		return tags, err
	}
	for _, de := range files {
		if de.Name() == "_blobs" {
			continue
		}
		tags = append(tags, de.Name())
	}
	return tags, nil
}

func writeServerError(err error, w http.ResponseWriter) {
	es := fmt.Sprintf("Unexpected error encountered: %s", err.Error())
	http.Error(w, es, 500)
}

func writeToFile(destFile string, w http.ResponseWriter, r *http.Request) {
	var f *os.File
	if _, statE := os.Stat(destFile); os.IsNotExist(statE) {
		innerF, err := os.OpenFile(destFile, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			writeServerError(err, w)
			return
		}
		f = innerF
	} else {
		innerF, err := os.OpenFile(destFile, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			writeServerError(err, w)
			return
		}
		err = os.Truncate(destFile, 0)
		if err != nil {
			writeServerError(err, w)
			return
		}
		f = innerF
	}
	total := r.ContentLength
	buf := make([]byte, 1024)
	for {
		n, err := r.Body.Read(buf)
		_, err2 := f.Write(buf[0:n])
		if err2 != nil {
			log.Printf("Failed to write buffer to file: %s", err2)
		}
		if err == io.EOF {
			break
		}
		total = total - int64(n)
		if total > 0 {
			for i := 0; i < 1024; i++ {
				buf[i] = 0
			}
		}
	}
	w.WriteHeader(201)
}

func readFile(path string) (bytes.Buffer, error) {
	var b bytes.Buffer
	f, err := os.Open(path)
	if err != nil {
		return b, err
	}
	_, readE := b.ReadFrom(f)
	if readE != nil {
		return bytes.Buffer{}, readE
	}
	return b, nil
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
	accept := r.Header.Get("Accept")

	log.Printf("Request details:")
	log.Printf("\tHost: %s", client)
	log.Printf("\tMethod: %s", method)
	log.Printf("\tURI: %s", uri)
	if conType != "" {
		log.Printf("\tContent-Type: %s", conType)
	}
	if accept != "" {
		log.Printf("\tAccept: %s", accept)
	}
}

func writeOciError(code string, message string, w http.ResponseWriter, statusCode int) {
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
		http.Error(w, err.Error(), 500)
	}
	http.Error(w, string(out[:]), statusCode)
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

func matches(pattern string, name string) bool {
	matched, err := regexp.MatchString(pattern, name)
	if err != nil {
		log.Printf("Error while parsing regex: %s", err.Error())
	}
	return matched
}

func fileExists(path string) (bool, error) {
	_, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		} else {
			return false, errors.New(fmt.Sprintf("Unexpected error while checking existence of %s: %s", path, err))
		}
	}
	return true, nil
}

func findManifest(rootDir string, name string, digest string) (string, error) {
	files, err := os.ReadDir(path.Join(rootDir, name))
	if err != nil {
		return "", err
	}
	for _, de := range files {
		if de.Name() == "_blobs" {
			continue
		}
		if de.IsDir() {
			manifestPath := path.Join(rootDir, name, de.Name(), "manifest.json")
			f, fE := os.Open(manifestPath)
			if fE != nil {
				return "", fE
			}
			var buf bytes.Buffer
			_, err := buf.ReadFrom(f)
			if err != nil {
				return "", err
			}
			h := sha256.Sum256(buf.Bytes())
			thisDigest := fmt.Sprintf("sha256:%x", h)
			if thisDigest == digest {
				return manifestPath, nil
			}
		}
	}
	return "", nil
}
