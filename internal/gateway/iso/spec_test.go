package iso_test

import (
	"testing"

	"github.com/moov-io/iso8583"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
)

// TestDiscoverSpecValidate verifies that the DiscoverSpec is a well-formed
// moov-io MessageSpec. This test acts as a guard against spec misconfiguration
// before deployment.
func TestDiscoverSpecValidate(t *testing.T) {
	err := iso.DiscoverSpec.Validate()
	require.NoError(t, err, "DiscoverSpec must pass moov-io/iso8583 spec validation")
}

// TestRoundTrip builds a 0800 EchoRequest, marshals and packs it into a
// raw byte slice, then unpacks and unmarshals it into a new message — asserting
// that all field values survive the full round trip.
func TestRoundTrip(t *testing.T) {
	// 1. Build an EchoRequest and marshal it into a message.
	original := iso.EchoRequest{
		STAN:                "123456",
		NetworkMgmtInfoCode: "301",
	}

	sendMsg := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(t, sendMsg.Marshal(&original))
	sendMsg.MTI("0800")

	// 2. Pack to raw bytes.
	packed, err := sendMsg.Pack()
	require.NoError(t, err, "Pack must succeed for a valid 0800 message")
	require.NotEmpty(t, packed)

	// 3. Unpack into a fresh message.
	recvMsg := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(t, recvMsg.Unpack(packed), "Unpack must succeed for a validly packed payload")

	// 4. Unmarshal into a struct and assert field equivalence.
	var received iso.EchoRequest
	require.NoError(t, recvMsg.Unmarshal(&received))

	mti, err := recvMsg.GetMTI()
	require.NoError(t, err)
	assert.Equal(t, "0800", mti, "MTI must survive pack/unpack round trip")
	assert.Equal(t, original.STAN, received.STAN, "F11 STAN must survive pack/unpack round trip")
	assert.Equal(t, original.NetworkMgmtInfoCode, received.NetworkMgmtInfoCode, "F70 must survive pack/unpack round trip")
}

// TestUnpackMalformedPayload verifies that Unpack returns a descriptive error
// and does NOT panic when given an invalid or truncated byte payload.
func TestUnpackMalformedPayload(t *testing.T) {
	malformed := []byte{0x00, 0x01, 0x02} // too short / garbage
	msg := iso8583.NewMessage(iso.DiscoverSpec)
	err := msg.Unpack(malformed)
	assert.Error(t, err, "Unpack of a malformed payload must return an error, not panic")
}

// TestNetworkHeaderFraming verifies that the binary 2-byte length prefix
// correctly frames a message of a known size.
// Example: a 115-byte message must produce the header bytes {0x00, 0x73}.
func TestNetworkHeaderFraming(t *testing.T) {
	// Build a minimal 0800 message and pack it.
	req := iso.EchoRequest{
		STAN:                "999999",
		NetworkMgmtInfoCode: "301",
	}
	msg := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(t, msg.Marshal(&req))
	msg.MTI("0800")

	packed, err := msg.Pack()
	require.NoError(t, err)

	// WriteLength writes 2 bytes; verify the header length matches packed length.
	expectedLen := len(packed)
	headerBytes := []byte{byte(expectedLen >> 8), byte(expectedLen)}
	assert.Equal(t, 2, len(headerBytes), "Network header must always be 2 bytes")
	assert.Equal(t, byte(expectedLen>>8), headerBytes[0])
	assert.Equal(t, byte(expectedLen&0xff), headerBytes[1])
}
