// Package iso implements ISO 8583 message handling for the Discover
// card network using github.com/moov-io/iso8583. See MOD-69.
package iso

import (
	"testing"

	"github.com/moov-io/iso8583"
	connection "github.com/moov-io/iso8583-connection"
	"go.uber.org/goleak"
)

// Message is an ISO 8583 message.
type Message struct {
	iso8583.Message
}

// NewConnection creates a new ISO 8583 connection.
func NewConnection() *connection.Connection {
	return &connection.Connection{}
}
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
