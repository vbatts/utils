package main

import (
	"flag"
	"log"
	"net/http"
	"path/filepath"
)

var (
	flRoot = flag.String("root", ".", "root path to serve")
	//flPrefix = flag.String("prefix", "", "prefix the served URL path")
	flBind = flag.String("b", "127.0.0.1", "addr to bind to")
	flPort = flag.String("p", "8888", "port to listen on")
)

func main() {
	flag.Parse()

	var err error
	*flRoot, err = filepath.Abs(*flRoot)
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/", http.FileServer(http.Dir(*flRoot)))
	log.Printf("Serving %s on %s:%s ...", *flRoot, *flBind, *flPort)
	log.Fatal(http.ListenAndServe(*flBind+":"+*flPort, nil))
}
