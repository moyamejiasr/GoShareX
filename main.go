package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
)

var (
	domain = flag.String("domain", ":80", "TCP address to listen to")
	output = flag.String("output", "out", "Uploads output directory")
	secret = flag.String("secret", "", "Secret key for allowing uploads (allow all if none)")
	iplist = flag.String("iplist", "127.0.0.1", "Ip list to allow displaying file-list")
	vPath  = flag.String("path", "/!/", "Virtual public path for preview")
	size   = flag.Int64("size", 10, "Max upload size in MB")
)

/*UploadFile uploads file to output if secret valid*/
func UploadFile(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(*size << 20)
	// Check secret key - hide if not match
	if *secret != r.FormValue("secret") {
		http.NotFound(w, r)
		return
	}

	// Get and read file from form data
	file, handler, err := r.FormFile("file")
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}

	// Get filename from MD5 sum - that way
	// we get rid of duplicates.
	fName := fmt.Sprintf("%x%s", md5.Sum(data),
		path.Ext(handler.Filename))
	// Write to file
	err = ioutil.WriteFile(path.Join(*output, fName),
		data, os.ModePerm)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
	// Display result
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

	// Static Handlers
	http.HandleFunc("/upload", UploadFile)

	// FServer Handler
	fServer := ListDirectory(http.FileServer(http.Dir(*output)))
	http.Handle(*vPath, http.StripPrefix(*vPath, fServer))

	fmt.Print("Listening on ", *domain, " address...")
	err := http.ListenAndServe(*domain, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println()
}
