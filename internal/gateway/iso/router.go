package iso

import (
	"fmt"

	"github.com/moov-io/iso8583"
)

// MessageHandler defines the interface for processing a specific ISO 8583
// Message Type Indicator (MTI). Implementations should unpack the message,
// apply business logic (or stubs), and return a response message.
type MessageHandler interface {
	Handle(msg *iso8583.Message) (*iso8583.Message, error)
}

// HandlerRegistry holds MTI -> handler mappings. It implements an extensibility
// pattern that allows adding new MTI handlers without modifying the core routing
// switch in handler.go.
type HandlerRegistry struct {
	handlers map[string]MessageHandler
}

// NewHandlerRegistry creates a new registry and pre-registers the standard
// MTIs required by MOD-74: 0800 (Echo), 0100 (Auth), 0120 (Post-Auth),
// and 0400 (Reversal).
func NewHandlerRegistry() *HandlerRegistry {
	r := &HandlerRegistry{
		handlers: make(map[string]MessageHandler),
	}

	r.Register("0800", EchoHandler{})
	r.Register("0100", AuthHandler{})
	r.Register("0120", PostAuthHandler{})
	r.Register("0400", ReversalHandler{})

	return r
}

// Register associates a MessageHandler with a specific MTI string.
// If a handler already exists for the given MTI, it will be overwritten.
func (r *HandlerRegistry) Register(mti string, handler MessageHandler) {
	r.handlers[mti] = handler
}

// Dispatch finds the registered handler for the given MTI and delegates
// the message handling to it. If no handler is registered for the MTI,
// it falls back to buildUnsupportedMTI0810 to return a protocol error.
func (r *HandlerRegistry) Dispatch(mti string, msg *iso8583.Message) (*iso8583.Message, error) {
	handler, ok := r.handlers[mti]
	if ok {
		return handler.Handle(msg)
	}

	return buildUnsupportedMTI0810(msg)
}

type responseCodeOnly struct {
	STAN         string `iso8583:"11"`
	ResponseCode string `iso8583:"39"`
}

// buildUnsupportedMTI0810 creates a protocol-safe response for unknown MTIs.
func buildUnsupportedMTI0810(msg *iso8583.Message) (*iso8583.Message, error) {
	var req struct {
		STAN string `iso8583:"11"`
	}
	if err := msg.Unmarshal(&req); err != nil {
		return nil, fmt.Errorf("buildUnsupportedMTI0810: unmarshal request: %w", err)
	}

	resp := responseCodeOnly{
		STAN:         req.STAN,
		ResponseCode: "12", // invalid transaction
	}

	out := iso8583.NewMessage(DiscoverSpec)
	if err := out.Marshal(&resp); err != nil {
		return nil, fmt.Errorf("buildUnsupportedMTI0810: marshal response: %w", err)
	}
	out.MTI("0810")

	return out, nil
}