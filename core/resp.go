package core

import (
	"errors"
	"fmt"
	"strconv"
)

// ErrIncomplete means more bytes are needed before a RESP value can be parsed.
var ErrIncomplete = errors.New("incomplete RESP data")

// readLength parses a decimal length (optional leading '-') up to CRLF.
// Returns the length, bytes consumed from data, and an error.
func readLength(data []byte) (int, int, error) {
	if len(data) == 0 {
		return 0, 0, ErrIncomplete
	}
	pos := 0
	neg := false
	if data[0] == '-' {
		neg = true
		pos++
		if pos >= len(data) {
			return 0, 0, ErrIncomplete
		}
	}
	if data[pos] < '0' || data[pos] > '9' {
		return 0, 0, fmt.Errorf("invalid length")
	}
	length := 0
	for pos < len(data) && data[pos] >= '0' && data[pos] <= '9' {
		length = length*10 + int(data[pos]-'0')
		pos++
	}
	if neg {
		length = -length
	}
	if pos >= len(data) || data[pos] != '\r' {
		return 0, 0, ErrIncomplete
	}
	if pos+1 >= len(data) || data[pos+1] != '\n' {
		return 0, 0, ErrIncomplete
	}
	return length, pos + 2, nil
}

// readSimpleString reads a RESP simple string: +payload\r\n
func readSimpleString(data []byte) (string, int, error) {
	if len(data) < 3 {
		return "", 0, ErrIncomplete
	}
	if data[0] != '+' {
		return "", 0, errors.New("not a simple string")
	}
	pos := 1
	for pos < len(data) && data[pos] != '\r' {
		pos++
	}
	if pos >= len(data) {
		return "", 0, ErrIncomplete
	}
	if pos+1 >= len(data) || data[pos+1] != '\n' {
		return "", 0, ErrIncomplete
	}
	return string(data[1:pos]), pos + 2, nil
}

// readError reads a RESP error string: -message\r\n
func readError(data []byte) (string, int, error) {
	if len(data) < 3 {
		return "", 0, ErrIncomplete
	}
	if data[0] != '-' {
		return "", 0, errors.New("not an error string")
	}
	pos := 1
	for pos < len(data) && data[pos] != '\r' {
		pos++
	}
	if pos >= len(data) {
		return "", 0, ErrIncomplete
	}
	if pos+1 >= len(data) || data[pos+1] != '\n' {
		return "", 0, ErrIncomplete
	}
	return string(data[1:pos]), pos + 2, nil
}

// readInt64 reads a RESP integer: :digits\r\n
func readInt64(data []byte) (int64, int, error) {
	if len(data) < 4 {
		return 0, 0, ErrIncomplete
	}
	if data[0] != ':' {
		return 0, 0, errors.New("not an integer")
	}
	pos := 1
	for pos < len(data) && data[pos] != '\r' {
		pos++
	}
	if pos+1 >= len(data) || data[pos+1] != '\n' {
		return 0, 0, ErrIncomplete
	}
	v, err := strconv.ParseInt(string(data[1:pos]), 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return v, pos + 2, nil
}

// readBulkString reads a RESP bulk string: $<len>\r\n<payload>\r\n (or $-1\r\n for null).
func readBulkString(data []byte) (interface{}, int, error) {
	if len(data) < 1 {
		return nil, 0, ErrIncomplete
	}
	if data[0] != '$' {
		return nil, 0, errors.New("not a bulk string")
	}
	pos := 1
	n, delta, err := readLength(data[pos:])
	if err != nil {
		return nil, 0, err
	}
	pos += delta
	if n == -1 {
		return nil, pos, nil
	}
	if n < 0 {
		return nil, 0, errors.New("invalid bulk string length")
	}
	if len(data) < pos+n+2 {
		return nil, 0, ErrIncomplete
	}
	if data[pos+n] != '\r' || data[pos+n+1] != '\n' {
		return nil, 0, errors.New("bulk string missing trailing CRLF")
	}
	return string(data[pos : pos+n]), pos + n + 2, nil
}

// readArray reads a RESP array: *<count>\r\n followed by count encoded values.
func readArray(data []byte) ([]interface{}, int, error) {
	if len(data) < 1 {
		return nil, 0, ErrIncomplete
	}
	if data[0] != '*' {
		return nil, 0, errors.New("not an array")
	}
	pos := 1
	n, delta, err := readLength(data[pos:])
	if err != nil {
		return nil, 0, err
	}
	pos += delta
	if n < 0 {
		return nil, 0, errors.New("invalid array length")
	}
	out := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		if pos >= len(data) {
			return nil, 0, ErrIncomplete
		}
		v, d, err := DecodeOne(data[pos:])
		if err != nil {
			return nil, 0, err
		}
		out = append(out, v)
		pos += d
	}
	return out, pos, nil
}

// DecodeOne decodes one RESP value from the start of data.
// Returns the value, total bytes consumed, and an error.
func DecodeOne(data []byte) (interface{}, int, error) {
	if len(data) == 0 {
		return nil, 0, errors.New("no data")
	}
	switch data[0] {
	case '+':
		return readSimpleString(data)
	case '-':
		return readError(data)
	case ':':
		return readInt64(data)
	case '$':
		return readBulkString(data)
	case '*':
		return readArray(data)
	default:
		return nil, 0, fmt.Errorf("unknown RESP type byte: %q", data[0])
	}
}

// Decode decodes a single RESP value that fully fits in data.
func Decode(data []byte) (interface{}, error) {
	if len(data) == 0 {
		return nil, errors.New("no data")
	}
	value, _, err := DecodeOne(data)
	return value, err
}

func DecodeArrayString(data []byte) ([]string, error) {
	value, err := Decode(data)
	if err != nil {
		return nil, err
	}

	ts, ok := value.([]interface{})
	if !ok {
		return nil, errors.New("expected RESP array")
	}

	tokens := make([]string, len(ts))
	for i := range tokens {
		s, ok := ts[i].(string)
		if !ok {
			return nil, errors.New("expected array of strings")
		}
		tokens[i] = s
	}

	return tokens, nil
}

func Encode(value interface{}, isSimple bool) []byte {
	switch v := value.(type) {
	case string:
		if isSimple {
			return []byte(fmt.Sprintf("+%s\r\n", v))
		}
		return []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(v), v))
	}

	return []byte{}
}
