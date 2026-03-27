package iso

import (
	"fmt"

	"github.com/moov-io/iso8583"
)

// HandleMessage receives a parsed, unpacked ISO 8583 message and returns an
// appropriate response message.
//
// Routing is done by MTI:
//   - "0800" → BuildEcho0810 (Network Management Request → Response)
//   - anything else → descriptive error
//
// The caller is responsible for packing the returned message and writing it
// to the TCP connection via NetworkHeader framing.
func HandleMessage(msg *iso8583.Message) (*iso8583.Message, error) {
	// mti, _ := msg.GetMTI()
	mti, err := msg.GetMTI()
	if err != nil {
		return nil, fmt.Errorf("HandleMessage: get MTI: %w", err)
	}
	

	switch mti {
	case "0800":
		return handleEchoRequest(msg)
	default:
		return nil, fmt.Errorf("HandleMessage: unsupported MTI %q", mti)
	}
}

// handleEchoRequest processes an 0800 Network Management Request.
func handleEchoRequest(msg *iso8583.Message) (*iso8583.Message, error) {
	var req EchoRequest
	
	// _ = msg.Unmarshal(&req)
	if err := msg.Unmarshal(&req); err != nil {
		return nil, fmt.Errorf("handleEchoRequest: unmarshal 0800: %w", err)
	}

	// resp, _ := BuildEcho0810(&req)
	resp, err := BuildEcho0810(&req)
	if err != nil {
		return nil, fmt.Errorf("handleEchoRequest: build 0810: %w", err)
	}

	return resp, nil
}
