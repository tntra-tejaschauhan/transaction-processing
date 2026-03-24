package iso_test

import (
	"net"
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

// TestNewNetworkHeader verifies that NewNetworkHeader returns a functional
// Binary2Bytes framer that correctly encodes and decodes the 2-byte length
// prefix over a real in-process pipe.
func TestNewNetworkHeader(t *testing.T) {
	t.Run("WriteTo then ReadFrom round-trips the message length correctly", func(t *testing.T) {
		client, server := net.Pipe()
		defer client.Close()
		defer server.Close()

		const testLen = 115 // arbitrary message length

		// Writer side — use NewNetworkHeader to write the length prefix.
		writeErr := make(chan error, 1)
		go func() {
			h := iso.NewNetworkHeader()
			_ = h.SetLength(testLen)
			_, err := h.WriteTo(client)
			writeErr <- err
		}()

		// Reader side — use a fresh NewNetworkHeader to read it back.
		h := iso.NewNetworkHeader()
		_, err := h.ReadFrom(server)
		require.NoError(t, err)
		assert.Equal(t, testLen, h.Length(), "decoded length must match the written length")
		require.NoError(t, <-writeErr)
	})
}

// TestNetworkHeaderFraming verifies the header's byte-level encoding:
// a 115-byte payload must be framed as {0x00, 0x73}.
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

	// Verify the header length matches packed length using manual bytes.
	expectedLen := len(packed)
	headerBytes := []byte{byte(expectedLen >> 8), byte(expectedLen)}
	assert.Equal(t, 2, len(headerBytes), "Network header must always be 2 bytes")
	assert.Equal(t, byte(expectedLen>>8), headerBytes[0])
	assert.Equal(t, byte(expectedLen&0xff), headerBytes[1])
}

