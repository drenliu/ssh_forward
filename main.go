package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"ssh_forward/internal/registry"
	"ssh_forward/internal/sshd"
	"ssh_forward/internal/store"
	"ssh_forward/internal/web"
)

func main() {
	dataDir := flag.String("data", "./data", "directory for SQLite and host key")
	sshAddr := flag.String("ssh", ":2222", "SSH listen address, e.g. :2222")
	httpAddr := flag.String("http", "127.0.0.1:8080", "web admin listen address")
	webUser := flag.String("web-user", "admin", "HTTP Basic user for web admin")
	webPass := flag.String("web-pass", "", "HTTP Basic password (required)")
	flag.Parse()

	if *webPass == "" {
		log.Fatal("set -web-pass for the web admin UI (HTTP Basic)")
	}

	if err := os.MkdirAll(*dataDir, 0700); err != nil {
		log.Fatal(err)
	}

	dbPath := filepath.Join(*dataDir, "app.db")
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	reg := registry.New()
	hostKeyPath := filepath.Join(*dataDir, "ssh_host_ed25519")

	go func() {
		if err := web.Serve(*httpAddr, *webUser, *webPass, st, reg); err != nil {
			log.Fatalf("web admin: %v", err)
		}
	}()

	log.Printf("web admin http://%s (user %q)", *httpAddr, *webUser)
	if err := sshd.Listen(*sshAddr, hostKeyPath, st, reg); err != nil {
		log.Fatal(err)
	}
}
