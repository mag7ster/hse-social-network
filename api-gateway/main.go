package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

func main() {
	log.Println("I am api gateway")

	userServiceAddr := flag.String("user-service", "", "Address of the user service")
	listenPort := flag.Int("port", 8090, "api gateway port")
	flag.Parse()

	if *userServiceAddr == "" || listenPort == nil {
		flag.Usage()
		os.Exit(1)
	}

	target, err := url.Parse(*userServiceAddr)
	if err != nil {
		log.Fatalf("Invalid user service URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	http.HandleFunc("/whoami", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	http.HandleFunc("/profile/update", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	http.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf(":%d", *listenPort)
	log.Printf("Starting api-gateway on %s, proxying to %s", addr, *userServiceAddr)
	if err = http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
