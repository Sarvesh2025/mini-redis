package core_test

import (
	"testing"

	"mini-redis/core"
)

// decodeFirst decodes data and returns the first decoded RESP value. The
// updated Decode supports pipelining and may return multiple values; tests
// here only assert one input at a time.
func decodeFirst(t *testing.T, data []byte) interface{} {
	t.Helper()
	values, err := core.Decode(data)
	if err != nil {
		t.Fatalf("decode %q: %v", string(data), err)
	}
	if len(values) == 0 {
		t.Fatalf("decode %q: no values", string(data))
	}
	return values[0]
}

func TestInt64(t *testing.T) {
	cases := map[string]int64{
		":0\r\n":    0,
		":1000\r\n": 1000,
	}
	for k, v := range cases {
		value := decodeFirst(t, []byte(k))
		ival, ok := value.(int64)
		if !ok || v != ival {
			t.Fatalf("decode %q: got %v (%T) want %v", k, value, value, v)
		}
	}
}

func TestSimpleString(t *testing.T) {
	v := decodeFirst(t, []byte("+OK\r\n"))
	if s, ok := v.(string); !ok || s != "OK" {
		t.Fatalf("got %v (%T) want OK", v, v)
	}
}

func TestBulkString(t *testing.T) {
	v := decodeFirst(t, []byte("$5\r\nhello\r\n"))
	if s, ok := v.(string); !ok || s != "hello" {
		t.Fatalf("got %v (%T) want hello", v, v)
	}
}

func TestArrayOfBulkStrings(t *testing.T) {
	raw := "*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"
	v := decodeFirst(t, []byte(raw))
	arr, ok := v.([]interface{})
	if !ok || len(arr) != 2 {
		t.Fatalf("got %v (%T)", v, v)
	}
	if arr[0] != "foo" || arr[1] != "bar" {
		t.Fatalf("got %#v", arr)
	}
}

func TestPipelinedArrays(t *testing.T) {
	raw := "*1\r\n$4\r\nPING\r\n*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n"
	values, err := core.Decode([]byte(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("expected 2 pipelined commands, got %d", len(values))
	}
	first, ok := values[0].([]interface{})
	if !ok || len(first) != 1 || first[0] != "PING" {
		t.Fatalf("first command = %#v", values[0])
	}
	second, ok := values[1].([]interface{})
	if !ok || len(second) != 3 || second[0] != "SET" || second[1] != "k" || second[2] != "v" {
		t.Fatalf("second command = %#v", values[1])
	}
}
