package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
	"unsafe"
)

var (
	domain = flag.String("domain", ":80", "TCP address to listen to")
	output = flag.String("output", "out", "Uploads output directory")
	secret = flag.String("secret", "", "Secret key for allowing uploads (allow all if none)")
	iplist = flag.String("iplist", "127.0.0.1", "Ip list to allow displaying file-list")
	vPath  = flag.String("path", "/!/", "Virtual public path for preview")
	size   = flag.Int64("size", 10, "Max upload size in MB stored in memory(rest is saved to disk)")
)

/*GenerateName return a filename base64 encoded from time*/
func GenerateName(str string) string {
	n := time.Now().UnixNano()
	name := (*[8]byte)(unsafe.Pointer(&n))
	return base64.RawURLEncoding.EncodeToString(name[:]) +
		path.Ext(str)
}

/*UploadFile uploads file to output if secret valid*/
func UploadFile(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(*size << 20)
	// Check secret key - hide if not match
	if err != nil || *secret != r.FormValue("secret") {
		http.NotFound(w, r)
		return
	}

	// Get file buffer from Form data
	buffer, handler, err := r.FormFile("file")
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
	defer buffer.Close()
	// Create local file
	fName := GenerateName(handler.Filename)
	file, err := os.Create(path.Join(*output, fName))
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
	defer file.Close()

	// Write file, print result and return
	_, err = io.Copy(file, buffer)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
	fmt.Fprintf(w, "http://%s%s", r.Host,
		path.Join(*vPath, fName))
}

/*ListDirectory display file list if true and secret valid*/
func ListDirectory(h http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.RawPath, "/") {
			// Remove port section from addr
			address := r.RemoteAddr
			if i := strings.Index(address, ":"); i > 0 {
				address = address[:i]
			}
			if !strings.Contains(*iplist, address) {
				http.NotFound(w, r)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

func main() {
	flag.Parse()
	// Check output exists
	os.MkdirAll(*output, os.ModePerm)

	// Static Handler
	http.HandleFunc("/upload", UploadFile)
	// FServer Handler
	fServer := ListDirectory(http.FileServer(http.Dir(*output)))
	http.Handle(*vPath, http.StripPrefix(*vPath, fServer))

	fmt.Print("Listening on ", *domain, " address...")
	err := http.ListenAndServe(*domain, nil)
	fmt.Println()

	if err != nil {
		fmt.Println(err)
	}
}
