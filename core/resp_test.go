package core_test

import (
	"testing"

	"mini-redis/core"
)

func TestInt64(t *testing.T) {
	cases := map[string]int64{
		":0\r\n":    0,
		":1000\r\n": 1000,
	}
	for k, v := range cases {
		value, err := core.Decode([]byte(k))
		if err != nil {
			t.Fatalf("decode %q: %v", k, err)
		}
		ival, ok := value.(int64)
		if !ok || v != ival {
			t.Fatalf("decode %q: got %v (%T) want %v", k, value, value, v)
		}
	}
}

func TestSimpleString(t *testing.T) {
	v, err := core.Decode([]byte("+OK\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := v.(string); !ok || s != "OK" {
		t.Fatalf("got %v (%T) want OK", v, v)
	}
}

func TestBulkString(t *testing.T) {
	v, err := core.Decode([]byte("$5\r\nhello\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := v.(string); !ok || s != "hello" {
		t.Fatalf("got %v (%T) want hello", v, v)
	}
}

func TestArrayOfBulkStrings(t *testing.T) {
	raw := "*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"
	v, err := core.Decode([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := v.([]interface{})
	if !ok || len(arr) != 2 {
		t.Fatalf("got %v (%T)", v, v)
	}
	if arr[0] != "foo" || arr[1] != "bar" {
		t.Fatalf("got %#v", arr)
	}
}
