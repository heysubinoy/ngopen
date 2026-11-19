package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

func TestAuthMessageRoundTrip(t *testing.T) {
	cases := []ProtocolAuthMessage{
		{AuthToken: "tok_123", Hostname: "AUTO"},
		{AuthToken: "", Hostname: ""},
		{AuthToken: "has:colons:inside", Hostname: "blue-whale-42.example.com"},
	}
	for _, want := range cases {
		frame, err := EncodeProtocolAuthMessage(want)
		if err != nil {
			t.Fatalf("encode %+v: %v", want, err)
		}
		got, err := DecodeProtocolAuthMessage(bytes.NewReader(frame))
		if err != nil {
			t.Fatalf("decode %+v: %v", want, err)
		}
		if got != want {
			t.Errorf("round trip mismatch: got %+v, want %+v", got, want)
		}
	}
}

func TestDecodeRejectsOversizedFrame(t *testing.T) {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, MaxAuthMessageSize+1)
	// No payload follows: the decoder must refuse before trying to allocate or read it.
	_, err := DecodeProtocolAuthMessage(bytes.NewReader(header))
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected oversized-frame error, got %v", err)
	}
}

func TestDecodeTruncatedFrame(t *testing.T) {
	frame, _ := EncodeProtocolAuthMessage(ProtocolAuthMessage{AuthToken: "abc", Hostname: "AUTO"})
	for _, cut := range []int{0, 2, 4, len(frame) - 1} {
		if _, err := DecodeProtocolAuthMessage(bytes.NewReader(frame[:cut])); err == nil {
			t.Errorf("expected error for frame truncated to %d bytes", cut)
		}
	}
}

func TestSendAuthResponse(t *testing.T) {
	cases := []struct {
		resp ProtocolAuthResponse
		want string
	}{
		{ProtocolAuthResponse{OK: true, Hostname: "red-fox-7.example.com"}, "OK:red-fox-7.example.com"},
		{ProtocolAuthResponse{OK: false, Reason: "Invalid token"}, "FAIL:Invalid token"},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		if err := SendAuthResponse(&buf, c.resp); err != nil {
			t.Fatal(err)
		}
		header := make([]byte, 4)
		if _, err := io.ReadFull(&buf, header); err != nil {
			t.Fatal(err)
		}
		if n := binary.BigEndian.Uint32(header); int(n) != len(c.want) {
			t.Errorf("length prefix = %d, want %d", n, len(c.want))
		}
		if got := buf.String(); got != c.want {
			t.Errorf("payload = %q, want %q", got, c.want)
		}
	}
}
