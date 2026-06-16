package terminal

import (
	"bytes"
	"testing"
)

func TestBinaryFrameRoundTrip(t *testing.T) {
	payload := []byte("hello pty world\n")
	frame := EncodeBinaryFrame(42, payload)

	if len(frame) != 8+len(payload) {
		t.Fatalf("unexpected frame size: got %d, want %d", len(frame), 8+len(payload))
	}

	seq, data, err := DecodeBinaryFrame(frame)
	if err != nil {
		t.Fatalf("DecodeBinaryFrame: %v", err)
	}
	if seq != 42 {
		t.Errorf("seq mismatch: got %d, want 42", seq)
	}
	if !bytes.Equal(data, payload) {
		t.Errorf("payload mismatch: got %q, want %q", data, payload)
	}
}

func TestBinaryFrameEmptyPayload(t *testing.T) {
	frame := EncodeBinaryFrame(0, nil)
	seq, data, err := DecodeBinaryFrame(frame)
	if err != nil {
		t.Fatalf("DecodeBinaryFrame: %v", err)
	}
	if seq != 0 {
		t.Errorf("seq mismatch: got %d, want 0", seq)
	}
	if len(data) != 0 {
		t.Errorf("data should be empty, got %d bytes", len(data))
	}
}

func TestBinaryFrameTooShort(t *testing.T) {
	_, _, err := DecodeBinaryFrame([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected error for short frame")
	}
}

func TestEncodeDecodeInput(t *testing.T) {
	raw := []byte(`{"type":"input","data":"aGVsbG8="}`)
	got, err := DecodeText(raw)
	if err != nil {
		t.Fatalf("DecodeText: %v", err)
	}
	in, ok := got.(*InputFrame)
	if !ok {
		t.Fatalf("expected *InputFrame, got %T", got)
	}
	if in.Data != "aGVsbG8=" {
		t.Errorf("payload mismatch: got %q", in.Data)
	}

	encoded, err := EncodeText(in)
	if err != nil {
		t.Fatalf("EncodeText: %v", err)
	}
	if !bytes.Contains(encoded, []byte(`"type":"input"`)) {
		t.Errorf("encoded output missing type field: %s", encoded)
	}
}

func TestEncodeDecodeResize(t *testing.T) {
	raw := []byte(`{"type":"resize","cols":120,"rows":40}`)
	got, err := DecodeText(raw)
	if err != nil {
		t.Fatalf("DecodeText: %v", err)
	}
	r, ok := got.(*ResizeFrame)
	if !ok {
		t.Fatalf("expected *ResizeFrame, got %T", got)
	}
	if r.Cols != 120 || r.Rows != 40 {
		t.Errorf("size mismatch: got %dx%d, want 120x40", r.Cols, r.Rows)
	}
}

func TestEncodeDecodeReplay(t *testing.T) {
	raw := []byte(`{"type":"replay","since_seq":12345}`)
	got, err := DecodeText(raw)
	if err != nil {
		t.Fatalf("DecodeText: %v", err)
	}
	rp, ok := got.(*ReplayFrame)
	if !ok {
		t.Fatalf("expected *ReplayFrame, got %T", got)
	}
	if rp.SinceSeq != 12345 {
		t.Errorf("since_seq mismatch: got %d", rp.SinceSeq)
	}
}

func TestDecodeUnknownType(t *testing.T) {
	raw := []byte(`{"type":"wat","foo":"bar"}`)
	if _, err := DecodeText(raw); err == nil {
		t.Fatal("expected error for unknown frame type")
	}
}

func TestDecodeMalformedJSON(t *testing.T) {
	raw := []byte(`{not json`)
	if _, err := DecodeText(raw); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}
