package terminal

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

// Frame types exchanged between the GoSite server and the xterm.js client.
const (
	// Server -> client.
	FrameReady = "ready"
	FrameExit  = "exit"
	FrameError = "error"
	FramePong  = "pong"

	// Client -> server.
	FrameInput  = "input"
	FrameResize = "resize"
	FramePing   = "ping"
	FrameReplay = "replay"
)

// Roles assigned by the server when a websocket attaches to a session.
const (
	RoleWriter = "writer"
	RoleReader = "reader"
)

// ReadyFrame is the first text frame sent after the websocket upgrade.
type ReadyFrame struct {
	Type           string    `json:"type"`
	SessionID      string    `json:"session_id"`
	Shell          string    `json:"shell"`
	Cwd            string    `json:"cwd"`
	Cols           int       `json:"cols"`
	Rows           int       `json:"rows"`
	Role           string    `json:"role"`
	BufferedBytes  int64     `json:"buffered_bytes"`
	FirstSeq       uint64    `json:"first_seq"`
	EndSeq         uint64    `json:"end_seq"`
	StickyTTL      string    `json:"sticky_ttl"`
	StartedAt      string    `json:"started_at,omitempty"`
}

// ExitFrame is sent when the underlying shell process terminates.
type ExitFrame struct {
	Type string `json:"type"`
	Code int    `json:"code"`
}

// ErrorFrame reports protocol or runtime errors to the client.
type ErrorFrame struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// PongFrame is the response to a client ping.
type PongFrame struct {
	Type string `json:"type"`
}

// InputFrame is the writer -> server stdin payload.
type InputFrame struct {
	Type string `json:"type"`
	Data string `json:"data"` // base64-encoded stdin bytes
}

// ResizeFrame reports new terminal dimensions.
type ResizeFrame struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// PingFrame is a heartbeat from the client.
type PingFrame struct {
	Type string `json:"type"`
}

// ReplayFrame is sent by a client after WS reconnect to fetch missed output.
type ReplayFrame struct {
	Type     string `json:"type"`
	SinceSeq uint64 `json:"since_seq"`
}

// EncodeBinaryFrame prefixes a chunk of PTY output with its monotonic sequence
// number so the client can dedup across reconnects.
//
// Wire format: [8 bytes big-endian seq][raw bytes]
func EncodeBinaryFrame(seq uint64, data []byte) []byte {
	buf := make([]byte, 8+len(data))
	binary.BigEndian.PutUint64(buf[:8], seq)
	copy(buf[8:], data)
	return buf
}

// DecodeBinaryFrame splits a binary frame into its sequence number and payload.
// Returns an error if the buffer is shorter than the 8-byte header.
func DecodeBinaryFrame(frame []byte) (seq uint64, data []byte, err error) {
	if len(frame) < 8 {
		return 0, nil, errors.New("binary frame too short")
	}
	seq = binary.BigEndian.Uint64(frame[:8])
	data = make([]byte, len(frame)-8)
	copy(data, frame[8:])
	return seq, data, nil
}

// EncodeText marshals any control frame struct to JSON.
func EncodeText(v interface{}) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode text frame: %w", err)
	}
	return b, nil
}

// DecodeText unmarshals a JSON text frame and returns the concrete payload based
// on the `type` field. Unknown types return an error.
func DecodeText(raw []byte) (interface{}, error) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode text frame: %w", err)
	}
	switch envelope.Type {
	case FrameReady:
		var f ReadyFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case FrameExit:
		var f ExitFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case FrameError:
		var f ErrorFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case FramePong:
		var f PongFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case FrameInput:
		var f InputFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case FrameResize:
		var f ResizeFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case FramePing:
		var f PingFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		return &f, nil
	case FrameReplay:
		var f ReplayFrame
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		return &f, nil
	default:
		return nil, fmt.Errorf("unknown frame type %q", envelope.Type)
	}
}
