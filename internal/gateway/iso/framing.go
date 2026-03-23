package iso

import "github.com/moov-io/iso8583/network"

// NewNetworkHeader returns a new Binary2Bytes framing header for the 2-byte
// big-endian length prefix used in all Discover TCP frames.
//
// The Header implements io.ReaderFrom (ReadFrom) and io.WriterTo (WriteTo),
// and is sourced directly from the moov-io/iso8583 library — do NOT implement
// a custom framer.
//
// Usage (in connection read path):
//
//	header := iso.NewNetworkHeader()
//	_, err := header.ReadFrom(conn)      // reads exactly 2 bytes
//	length := header.Length()
//	buf := make([]byte, length)
//	_, err = io.ReadFull(conn, buf)
//
// Usage (in connection write path):
//
//	header := iso.NewNetworkHeader()
//	_ = header.SetLength(len(data))
//	_, err := header.WriteTo(conn)       // writes exactly 2 bytes
//	_, err = conn.Write(data)
func NewNetworkHeader() *network.Binary2Bytes {
	return network.NewBinary2BytesHeader()
}
