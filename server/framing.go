package server

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/http"
)

// writeFramedRequest writes an HTTP request into the given stream.
func WriteFramedRequest(stream net.Conn, req *http.Request) error {
	var buf bytes.Buffer
	if err := req.Write(&buf); err != nil {
		return err
	}
	data := buf.Bytes()
	length := uint32(len(data))
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, length)
	_, err := stream.Write(append(header, data...))
	return err
}

// readFramedResponse reads a framed HTTP response from the given stream.
func ReadFramedResponse(stream net.Conn, req *http.Request) (*http.Response, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(stream, header); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header)
	body := make([]byte, length)
	if _, err := io.ReadFull(stream, body); err != nil {
		return nil, err
	}
	return http.ReadResponse(bufio.NewReader(bytes.NewReader(body)), req)
}