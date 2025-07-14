package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

type ProtocolAuthMessage struct {
	AuthToken string
	Hostname  string
}

type ProtocolAuthResponse struct {
	OK       bool
	Hostname string // assigned hostname if OK
	Reason   string // failure reason if not OK
}

// --- Input (from client) ---
func ParseAuthRequest(r io.Reader) (ProtocolAuthMessage, error) {
	return DecodeProtocolAuthMessage(r)
}

// --- Output (to client) ---
func SendAuthResponse(w io.Writer, resp ProtocolAuthResponse) error {
	var payload string
	if resp.OK {
		payload = fmt.Sprintf("OK:%s", resp.Hostname)
	} else {
		payload = fmt.Sprintf("FAIL:%s", resp.Reason)
	}
	length := uint32(len(payload))
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, length)
	_, err := w.Write(append(header, []byte(payload)...))
	return err
}

func EncodeProtocolAuthMessage(msg ProtocolAuthMessage) ([]byte, error) {
	payload := fmt.Sprintf("AUTHTOKEN:%s\nHOSTNAME:%s\n", msg.AuthToken, msg.Hostname)
	length := uint32(len(payload))
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, length)
	return append(header, []byte(payload)...), nil
}

func DecodeProtocolAuthMessage(r io.Reader) (ProtocolAuthMessage, error) {
	var msg ProtocolAuthMessage
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return msg, fmt.Errorf("failed to read auth header: %w", err)
	}
	length := binary.BigEndian.Uint32(header)
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return msg, fmt.Errorf("failed to read auth payload: %w", err)
	}
	lines := string(buf)
	for _, line := range splitLines(lines) {
		if len(line) == 0 {
			continue
		}
		if k, v, ok := parseKeyValue(line); ok {
			switch k {
			case "AUTHTOKEN":
				msg.AuthToken = v
			case "HOSTNAME":
				msg.Hostname = v
			}
		}
	}
	return msg, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func parseKeyValue(line string) (string, string, bool) {
	for i := 0; i < len(line); i++ {
		if line[i] == ':' {
			return line[:i], line[i+1:], true
		}
	}
	return "", "", false
}
