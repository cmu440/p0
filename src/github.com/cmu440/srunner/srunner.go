package main

import (
	"fmt"
	"github.com/cmu440/p0"
)

const defaultPort = 9999

func main() {
	// Initialize the server.
	server := p0.New()
	if server == nil {
		fmt.Println("New() returned a nil server. Exiting...")
		return
	}

	// Start the server and continue listening for client connections in the background.
	if err := server.Start(defaultPort); err != nil {
		fmt.Printf("MultiEchoServer could not be started: %s\n", err)
		return
	}

	fmt.Printf("Started MultiEchoServer on port %d...\n", defaultPort)

	// Block forever.
	select {}
}
