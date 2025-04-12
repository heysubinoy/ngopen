package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

type ProtocolMessage struct {
	Protocol  string
	Hostname  string
	AuthToken string
	RemoteAddr string
}




func ParseProtocolMessage(reader io.Reader) (ProtocolMessage, error) {
	var message ProtocolMessage

	// Read 4-byte length
	var length uint32
	if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
		return message, fmt.Errorf("failed to read length: %w", err)
	}

	// Read exactly 'length' bytes
	buf := make([]byte, length)
	_, err := io.ReadFull(reader, buf)
	if err != nil {
		return message, fmt.Errorf("failed to read full payload: %w", err)
	}

	// Now parse it like before
	lines := strings.Split(string(buf), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return message, fmt.Errorf("invalid line: %s", line)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "PROTOCOL":
			message.Protocol = value
		case "HOSTNAME":
			message.Hostname = value
		case "REMOTEADDR":
			message.RemoteAddr = value
		case "AUTHTOKEN":
			message.AuthToken = value
		}
	}

	return message, nil
}
