package iso

import (
	"fmt"
	"github.com/moov-io/iso8583"
)

// BuildEcho0810 constructs an ISO 8583 0810 Network Management Response
// message from the values in the incoming 0800 EchoRequest.
//
// It handles both network management variants:
//   - F70=001 (Sign-on): sent by acquirers to initiate a session with the network.
//   - F70=301 (Echo):    a keep-alive / connectivity check.
//
// In both cases the response behaviour is identical:
//   - Sets MTI to "0810"
//   - Echoes F11 (STAN) unchanged from the request
//   - Sets F39 (ResponseCode) to "00" (approved)
//   - Echoes F70 (NetworkMgmtInfoCode) unchanged from the request
func BuildEcho0810(req *EchoRequest) (*iso8583.Message, error) {
	resp := EchoResponse{
		STAN:                req.STAN,
		ResponseCode:        "00",
		NetworkMgmtInfoCode: req.NetworkMgmtInfoCode,
	}

	msg := iso8583.NewMessage(DiscoverSpec)

	// _ = msg.Marshal(&resp)
	if err := msg.Marshal(&resp); err != nil {
		return nil, fmt.Errorf("BuildEcho0810: marshal response: %w", err)
	}

	msg.MTI("0810")

	return msg, nil
}
