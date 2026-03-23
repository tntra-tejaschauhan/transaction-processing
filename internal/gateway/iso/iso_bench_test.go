package iso_test

import (
	"testing"

	"github.com/moov-io/iso8583"
	"github.com/stretchr/testify/require"

	"github.com/PayWithSpireInc/transaction-processing/internal/gateway/iso"
)

// BenchmarkPack measures the time to marshal+pack an 0800 EchoRequest into
// a raw byte slice. This reflects the on-hot-path encoding cost per message.
func BenchmarkPack(b *testing.B) {
	req := iso.EchoRequest{
		STAN:                "123456",
		NetworkMgmtInfoCode: "301",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := iso8583.NewMessage(iso.DiscoverSpec)
		if err := msg.Marshal(&req); err != nil {
			b.Fatal(err)
		}
		msg.MTI("0800")
		if _, err := msg.Pack(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUnpack measures the time to unpack a raw 0800 byte slice and
// unmarshal it into an EchoRequest struct.
func BenchmarkUnpack(b *testing.B) {
	// Pre-pack once to get the canonical raw bytes.
	req := iso.EchoRequest{
		STAN:                "123456",
		NetworkMgmtInfoCode: "301",
	}
	setup := iso8583.NewMessage(iso.DiscoverSpec)
	require.NoError(b, setup.Marshal(&req))
	setup.MTI("0800")
	packed, err := setup.Pack()
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := iso8583.NewMessage(iso.DiscoverSpec)
		if err := msg.Unpack(packed); err != nil {
			b.Fatal(err)
		}
		var out iso.EchoRequest
		if err := msg.Unmarshal(&out); err != nil {
			b.Fatal(err)
		}
	}
}
