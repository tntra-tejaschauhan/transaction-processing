// Command simulator is a CLI tool for manually testing the ISO gateway.
// Usage: ./simulator --host localhost --port 8583
//
// The simulator connects to a running gateway, sends an 0800 echo request,
// and prints the 0810 response.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/PayWithSpireInc/transaction-processing/api/gateway"
)

func main1() {
	// Parse command-line flags.
	host := flag.String("host", "localhost", "gateway host")
	port := flag.String("port", "8583", "gateway port")
	stan := flag.String("stan", "123456", "Systems Trace Audit Number (STAN)")
	code := flag.String("code", "301", "Network Management Info Code")
	flag.Parse()

	addr := fmt.Sprintf("%s:%s", *host, *port)
	log.Printf("Connecting to gateway at %s...", addr)

	// Dial the gateway.
	client, err := gateway.New(addr)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	log.Printf("Connected. Sending echo request (STAN=%s, Code=%s)...", *stan, *code)

	// Send echo request with a 30-second timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.SendEcho(ctx, *stan, *code)
	if err != nil {
		log.Fatalf("SendEcho failed: %v", err)
	}

	// Print the response.
	sep := strings.Repeat("=", 60)
	fmt.Println("\n" + sep)
	fmt.Println("Echo Response (0810)")
	fmt.Println(sep)
	fmt.Printf("STAN:               %s\n", resp.STAN)
	fmt.Printf("ResponseCode:       %s\n", resp.ResponseCode)
	fmt.Printf("NetworkMgmtCode:    %s\n", resp.NetworkMgmtInfoCode)
	fmt.Println(sep + "\n")

	log.Println("Success!")
}
