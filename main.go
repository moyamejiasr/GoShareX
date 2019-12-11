package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
	"unsafe"
)

var (
	domain    = flag.String("domain", ":80", "TCP address to listen to")
	output    = flag.String("output", "out", "Uploads output directory")
	secret    = flag.String("secret", "", "Secret key for allowing uploads (allow all if none)")
	whitelist = flag.String("whitelist", "127.0.0.1", "Ip list to allow displaying file-list")
	virPath   = flag.String("path", "/!/", "Virtual public path for preview")
	errPage   = flag.String("error", "", "Custom error 404 html page to display")
	connLog   = flag.Bool("log", false, "Log image preview & upload requests to default output")
	size      = flag.Int64("size", 10, "Max upload size in MB stored in memory(rest is saved to disk)")
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
	value := r.FormValue("secret")
	if err != nil || *secret != value {
		http.NotFound(w, r)
		if *connLog { // Log request
			log.Printf("UPLOAD[%s]>> SECRET_FAIL:%s RET\n",
				r.RemoteAddr, value)
		}
		return
	}

	// Get file buffer from Form data
	buffer, handler, err := r.FormFile("file")
	if err != nil {
		_, _ = fmt.Fprintln(w, err)
		if *connLog { // Log request
			log.Printf("UPLOAD[%s]>> BUFFER_FAIL RET\n",
				r.RemoteAddr)
		}
		return
	}
	defer buffer.Close()
	// Create local file
	fName := GenerateName(handler.Filename)
	file, err := os.Create(path.Join(*output, fName))
	if err != nil {
		_, _ = fmt.Fprintln(w, err)
		if *connLog { // Log request
			log.Printf("UPLOAD[%s]>> OSMAKE_FAIL:%s RET\n",
				r.RemoteAddr, fName)
		}
		return
	}
	defer file.Close()

	// Write file, print result and return
	_, err = io.Copy(file, buffer)
	if err != nil {
		_, _ = fmt.Fprintln(w, err)
		if *connLog { // Log request
			log.Printf("UPLOAD[%s]>> FWRITE_FAIL:%s RET\n",
				r.RemoteAddr, fName)
		}
		return
	}
	_, _ = fmt.Fprintf(w, "http://%s%s", r.Host,
		path.Join(*virPath, fName))
	if *connLog { // Log request
		log.Printf("UPLOAD[%s]>> SUCCESS:%s RET\n",
			r.RemoteAddr, fName)
	}
}

/*ListDirectory display file list if true and secret valid*/
func ListDirectory(h http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.RawPath, "/") ||
			len(r.URL.RawPath) == 0 {
			// Remove port section from addr
			address := r.RemoteAddr
			if i := strings.Index(address, ":"); i > 0 {
				address = address[:i]
			}
			if !strings.Contains(*whitelist, address) {
				if len(*errPage) == 0 {
					http.NotFound(w, r)
				} else {
					http.ServeFile(w, r, *errPage)
				}
				if *connLog { // Log request
					log.Printf("ACCESS[%s]>> DIRLS_FAIL RET\n",
						r.RemoteAddr)
				}
				return
			}
		}
		h.ServeHTTP(w, r)
		if *connLog { // Log request
			log.Printf("ACCESS[%s]>> SERVE:%s RET\n",
				r.RemoteAddr, r.URL.RawPath)
		}
	})
}

func main() {
	flag.Parse()
	// Check output exists
	_ = os.MkdirAll(*output, os.ModePerm)

	// Static Handler
	http.HandleFunc("/upload", UploadFile)
	// FServer Handler
	fServer := ListDirectory(http.FileServer(http.Dir(*output)))
	http.Handle(*virPath, http.StripPrefix(*virPath, fServer))

	fmt.Print("Listening on ", *domain, " address...")
	err := http.ListenAndServe(*domain, nil)
	fmt.Println()

	if err != nil {
		fmt.Println(err)
	}
}
