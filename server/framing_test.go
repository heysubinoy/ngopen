package server

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

// TestFramedRequestResponseRoundTrip drives both halves of the tunnel framing
// over an in-memory connection: the server writes a framed request, the
// "client" side decodes it, replies with a framed response, and the server
// parses that response.
func TestFramedRequestResponseRoundTrip(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	defer serverSide.Close()
	defer clientSide.Close()

	req, err := http.NewRequest(http.MethodPost, "http://blue-whale-1.example.com/api/items?x=1", strings.NewReader(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Test", "yes")

	clientErr := make(chan error, 1)
	go func() {
		clientErr <- func() error {
			header := make([]byte, 4)
			if _, err := io.ReadFull(clientSide, header); err != nil {
				return err
			}
			payload := make([]byte, binary.BigEndian.Uint32(header))
			if _, err := io.ReadFull(clientSide, payload); err != nil {
				return err
			}
			got, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(payload)))
			if err != nil {
				return err
			}
			body, _ := io.ReadAll(got.Body)
			if got.Method != http.MethodPost || got.URL.RequestURI() != "/api/items?x=1" ||
				got.Header.Get("X-Test") != "yes" || string(body) != `{"a":1}` {
				t.Errorf("client decoded unexpected request: %s %s %v %q", got.Method, got.URL, got.Header, body)
			}

			resp := &http.Response{
				StatusCode:    http.StatusCreated,
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        http.Header{"Content-Type": {"application/json"}},
				Body:          io.NopCloser(strings.NewReader(`{"ok":true}`)),
				ContentLength: int64(len(`{"ok":true}`)),
			}
			var buf bytes.Buffer
			if err := resp.Write(&buf); err != nil {
				return err
			}
			frame := make([]byte, 4, 4+buf.Len())
			binary.BigEndian.PutUint32(frame, uint32(buf.Len()))
			_, err = clientSide.Write(append(frame, buf.Bytes()...))
			return err
		}()
	}()

	if err := WriteFramedRequest(serverSide, req); err != nil {
		t.Fatalf("WriteFramedRequest: %v", err)
	}
	resp, err := ReadFramedResponse(serverSide, req)
	if err != nil {
		t.Fatalf("ReadFramedResponse: %v", err)
	}
	defer resp.Body.Close()
	if err := <-clientErr; err != nil {
		t.Fatalf("client side: %v", err)
	}

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated || resp.Header.Get("Content-Type") != "application/json" || string(body) != `{"ok":true}` {
		t.Errorf("unexpected response: %d %v %q", resp.StatusCode, resp.Header, body)
	}
}
