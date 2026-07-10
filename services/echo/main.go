package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	version := os.Getenv("VERSION")
	if version == "" {
		version = "dev"
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("echo listening on %s (version %s)", addr, version)
	if err := http.ListenAndServe(addr, newHandler(version)); err != nil {
		log.Fatal(err)
	}
}
