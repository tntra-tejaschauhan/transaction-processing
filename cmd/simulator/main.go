// Command simulator is a standalone CLI tool for manually testing the ISO gateway.
// It connects to the gateway via TCP, sends a valid 0800 Network Management
// Request (echo) message with a random STAN, and prints the 0810 response
// returned by the server.
//
// Usage:
//
//	./bin/simulator --host localhost --port 8583
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/moov-io/iso8583"

	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
)

func main() {
	// ── 1. Parse CLI flags ──────────────────────────────────────────────────────
	host := flag.String("host", "localhost", "Gateway host to connect to")
	port := flag.Int("port", 8583, "Gateway port to connect to")
	flag.Parse()

	addr := fmt.Sprintf("%s:%d", *host, *port)

	// ── 2. Establish TCP connection ─────────────────────────────────────────────
	log.Printf("Connecting to gateway at %s …", addr)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: could not connect to %s: %v\n", addr, err)
		os.Exit(1)
	}
	defer conn.Close()
	log.Printf("Connected to %s", addr)

	// ── 3. Build an 0800 Network Management Request ─────────────────────────────
	// Generate a random 6-digit STAN, e.g. "042817".
	// #nosec G404 — math/rand is fine for a non-security STAN trace number.
	stan := fmt.Sprintf("%06d", rand.New(rand.NewSource(time.Now().UnixNano())).Intn(1_000_000))

	req := iso.EchoRequest{
		STAN:                stan,
		NetworkMgmtInfoCode: "301", // Discover echo code
	}
	log.Printf("Sending 0800 echo request — STAN: %s", stan)

	// Marshal request into an ISO 8583 message.
	reqMsg := iso8583.NewMessage(iso.DiscoverSpec)
	if err := reqMsg.Marshal(&req); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: marshal 0800 request: %v\n", err)
		os.Exit(1)
	}
	reqMsg.MTI("0800")

	// Pack message into raw bytes.
	packed, err := reqMsg.Pack()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: pack 0800 message: %v\n", err)
		os.Exit(1)
	}

	// ── 4. Write framed 0800 to the connection ──────────────────────────────────
	// The 2-byte binary length prefix is handled by our shared NetworkHeader.
	sendHeader := iso.NewNetworkHeader()
	if err := sendHeader.SetLength(len(packed)); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: set frame length: %v\n", err)
		os.Exit(1)
	}
	if _, err := sendHeader.WriteTo(conn); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: write frame header: %v\n", err)
		os.Exit(1)
	}
	if _, err := conn.Write(packed); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: write message payload: %v\n", err)
		os.Exit(1)
	}

	// ── 5. Read framed 0810 response ────────────────────────────────────────────
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: set read deadline: %v\n", err)
		os.Exit(1)
	}

	recvHeader := iso.NewNetworkHeader()
	if _, err := recvHeader.ReadFrom(conn); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: read response frame header: %v\n", err)
		os.Exit(1)
	}

	respBuf := make([]byte, recvHeader.Length())
	if _, err := io.ReadFull(conn, respBuf); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: read response payload: %v\n", err)
		os.Exit(1)
	}

	// ── 6. Parse and print response ─────────────────────────────────────────────
	respMsg := iso8583.NewMessage(iso.DiscoverSpec)
	if err := respMsg.Unpack(respBuf); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: unpack 0810 response: %v\n", err)
		os.Exit(1)
	}

	mti, err := respMsg.GetMTI()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: get response MTI: %v\n", err)
		os.Exit(1)
	}

	var resp iso.EchoResponse
	if err := respMsg.Unmarshal(&resp); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: unmarshal 0810 response: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("─────────────────────────────────")
	fmt.Printf("  MTI            : %s\n", mti)
	fmt.Printf("  STAN (Field 11): %s\n", resp.STAN)
	fmt.Printf("  Response Code  : %s\n", resp.ResponseCode)
	fmt.Println("─────────────────────────────────")

	if resp.ResponseCode != "00" {
		fmt.Fprintf(os.Stderr, "WARN: gateway returned non-approved response code %q\n", resp.ResponseCode)
		os.Exit(1)
	}

	log.Printf("Success — gateway echoed MTI 0810 with response code 00")
}
