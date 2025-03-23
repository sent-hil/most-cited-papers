package main

import (
	"flag"
	"log"
)

func main() {
	// Define command line flags
	dbPath := flag.String("db", "paper_cache.db", "Path to the SQLite database file")
	addr := flag.String("addr", ":9001", "HTTP server address")
	flag.Parse()

	// Create a new UI server
	server, err := NewUIServer(*dbPath)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	log.Printf("Starting UI server at %s", *addr)
	log.Printf("Database: %s", *dbPath)
	log.Printf("Open your browser at http://localhost%s", *addr)

	if err := server.Start(*addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
