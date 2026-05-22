package core

import (
	"bytes"
	"strings"
	"testing"

	"mini-redis/config"
)

type mockConn struct {
	bytes.Buffer
}

func (m *mockConn) Read(p []byte) (int, error) {
	return 0, nil
}

func resetStoreForTest() {
	storeMu.Lock()
	store = make(map[string]*Obj)
	keyVersions = make(map[string]uint64)
	storeMu.Unlock()
}

func setup() {
	config.KeysLimit = 1000
	resetStoreForTest()
}

func TestMULTI(t *testing.T) {
	setup()
	ctx := NewClientContext()
	conn := &mockConn{}

	cmds := RedisCmds{
		{Cmd: "MULTI", Args: nil},
	}
	if err := EvalAndRespond(cmds, conn, ctx); err != nil {
		t.Fatal(err)
	}
	if !ctx.InMulti {
		t.Fatal("expected InMulti to be true")
	}
	if !strings.Contains(conn.String(), "+OK") {
		t.Fatalf("expected +OK, got %q", conn.String())
	}
}

func TestMULTINested(t *testing.T) {
	setup()
	ctx := NewClientContext()
	conn := &mockConn{}

	ctx.InMulti = true
	cmds := RedisCmds{
		{Cmd: "MULTI", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)
	if !strings.Contains(conn.String(), "ERR MULTI calls can not be nested") {
		t.Fatalf("expected nested MULTI error, got %q", conn.String())
	}
}

func TestEXECWithoutMULTI(t *testing.T) {
	setup()
	ctx := NewClientContext()
	conn := &mockConn{}

	cmds := RedisCmds{
		{Cmd: "EXEC", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)
	if !strings.Contains(conn.String(), "ERR EXEC without MULTI") {
		t.Fatalf("expected EXEC without MULTI error, got %q", conn.String())
	}
}

func TestDISCARDWithoutMULTI(t *testing.T) {
	setup()
	ctx := NewClientContext()
	conn := &mockConn{}

	cmds := RedisCmds{
		{Cmd: "DISCARD", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)
	if !strings.Contains(conn.String(), "ERR DISCARD without MULTI") {
		t.Fatalf("expected DISCARD without MULTI error, got %q", conn.String())
	}
}

func TestQueueingInMULTI(t *testing.T) {
	setup()
	ctx := NewClientContext()
	conn := &mockConn{}

	cmds := RedisCmds{
		{Cmd: "MULTI", Args: nil},
		{Cmd: "SET", Args: []string{"k1", "v1"}},
		{Cmd: "SET", Args: []string{"k2", "v2"}},
		{Cmd: "GET", Args: []string{"k1"}},
	}
	EvalAndRespond(cmds, conn, ctx)

	if len(ctx.TxQueue) != 3 {
		t.Fatalf("expected 3 queued commands, got %d", len(ctx.TxQueue))
	}

	resp := conn.String()
	queuedCount := strings.Count(resp, "+QUEUED")
	if queuedCount != 3 {
		t.Fatalf("expected 3 QUEUED responses, got %d in %q", queuedCount, resp)
	}

	if Get("k1") != nil {
		t.Fatal("SET should not execute during MULTI, but key exists")
	}
}

func TestMULTIEXEC(t *testing.T) {
	setup()
	ctx := NewClientContext()
	conn := &mockConn{}

	cmds := RedisCmds{
		{Cmd: "MULTI", Args: nil},
		{Cmd: "SET", Args: []string{"k1", "v1"}},
		{Cmd: "SET", Args: []string{"k2", "v2"}},
		{Cmd: "GET", Args: []string{"k1"}},
		{Cmd: "EXEC", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)

	if ctx.InMulti {
		t.Fatal("expected InMulti to be false after EXEC")
	}

	obj := Get("k1")
	if obj == nil || obj.Value != "v1" {
		t.Fatalf("expected k1=v1, got %v", obj)
	}
	obj = Get("k2")
	if obj == nil || obj.Value != "v2" {
		t.Fatalf("expected k2=v2, got %v", obj)
	}

	resp := conn.String()
	if !strings.Contains(resp, "*3") {
		t.Fatalf("expected EXEC response array of 3, got %q", resp)
	}
}

func TestDISCARD(t *testing.T) {
	setup()
	ctx := NewClientContext()
	conn := &mockConn{}

	cmds := RedisCmds{
		{Cmd: "MULTI", Args: nil},
		{Cmd: "SET", Args: []string{"k1", "v1"}},
		{Cmd: "DISCARD", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)

	if ctx.InMulti {
		t.Fatal("expected InMulti to be false after DISCARD")
	}
	if len(ctx.TxQueue) != 0 {
		t.Fatalf("expected empty queue after DISCARD, got %d", len(ctx.TxQueue))
	}
	if Get("k1") != nil {
		t.Fatal("DISCARD should have prevented SET from executing")
	}
}

func TestWATCH(t *testing.T) {
	setup()
	Put("balance", NewObj("100", -1))

	ctx := NewClientContext()
	conn := &mockConn{}

	cmds := RedisCmds{
		{Cmd: "WATCH", Args: []string{"balance"}},
	}
	EvalAndRespond(cmds, conn, ctx)

	if ctx.WatchedKeys == nil {
		t.Fatal("expected WatchedKeys to be initialized")
	}
	if _, ok := ctx.WatchedKeys["balance"]; !ok {
		t.Fatal("expected 'balance' in WatchedKeys")
	}
}

func TestWATCHInsideMULTI(t *testing.T) {
	setup()
	ctx := NewClientContext()
	ctx.InMulti = true
	conn := &mockConn{}

	cmds := RedisCmds{
		{Cmd: "WATCH", Args: []string{"k"}},
	}
	EvalAndRespond(cmds, conn, ctx)
	if !strings.Contains(conn.String(), "ERR WATCH inside MULTI is not allowed") {
		t.Fatalf("expected error, got %q", conn.String())
	}
}

func TestWATCHAbort(t *testing.T) {
	setup()
	Put("balance", NewObj("100", -1))

	ctx := NewClientContext()
	conn := &mockConn{}

	// WATCH balance
	cmds := RedisCmds{
		{Cmd: "WATCH", Args: []string{"balance"}},
		{Cmd: "MULTI", Args: nil},
		{Cmd: "SET", Args: []string{"balance", "200"}},
	}
	EvalAndRespond(cmds, conn, ctx)

	// Simulate another client modifying the key
	Put("balance", NewObj("50", -1))

	// Now EXEC — should abort because balance version changed
	conn.Reset()
	cmds = RedisCmds{
		{Cmd: "EXEC", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)

	resp := conn.String()
	if !strings.Contains(resp, "*-1") {
		t.Fatalf("expected nil array (WATCH abort), got %q", resp)
	}

	obj := Get("balance")
	if obj == nil || obj.Value != "50" {
		t.Fatalf("expected balance=50 (from other client), got %v", obj)
	}

	if ctx.InMulti {
		t.Fatal("expected InMulti to be false after aborted EXEC")
	}
}

func TestWATCHNoConflict(t *testing.T) {
	setup()
	Put("balance", NewObj("100", -1))

	ctx := NewClientContext()
	conn := &mockConn{}

	cmds := RedisCmds{
		{Cmd: "WATCH", Args: []string{"balance"}},
		{Cmd: "MULTI", Args: nil},
		{Cmd: "SET", Args: []string{"balance", "200"}},
		{Cmd: "EXEC", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)

	obj := Get("balance")
	if obj == nil || obj.Value != "200" {
		t.Fatalf("expected balance=200 (no conflict), got %v", obj)
	}
}

func TestWATCHMultipleKeys(t *testing.T) {
	setup()
	Put("a", NewObj("1", -1))
	Put("b", NewObj("2", -1))

	ctx := NewClientContext()
	conn := &mockConn{}

	cmds := RedisCmds{
		{Cmd: "WATCH", Args: []string{"a", "b"}},
		{Cmd: "MULTI", Args: nil},
		{Cmd: "SET", Args: []string{"a", "10"}},
	}
	EvalAndRespond(cmds, conn, ctx)

	// Modify only "b" externally
	Put("b", NewObj("20", -1))

	conn.Reset()
	cmds = RedisCmds{
		{Cmd: "EXEC", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)

	if !strings.Contains(conn.String(), "*-1") {
		t.Fatalf("expected WATCH abort (b changed), got %q", conn.String())
	}

	obj := Get("a")
	if obj == nil || obj.Value != "1" {
		t.Fatalf("expected a=1 (aborted), got %v", obj)
	}
}

func TestEXECAtomicDEL(t *testing.T) {
	setup()
	Put("x", NewObj("1", -1))
	Put("y", NewObj("2", -1))

	ctx := NewClientContext()
	conn := &mockConn{}

	cmds := RedisCmds{
		{Cmd: "MULTI", Args: nil},
		{Cmd: "DEL", Args: []string{"x", "y"}},
		{Cmd: "GET", Args: []string{"x"}},
		{Cmd: "EXEC", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)

	if Get("x") != nil || Get("y") != nil {
		t.Fatal("expected x and y to be deleted")
	}

	resp := conn.String()
	if !strings.Contains(resp, "*2") {
		t.Fatalf("expected array of 2, got %q", resp)
	}
}

func TestEXECWithEXPIRE(t *testing.T) {
	setup()
	Put("ttlkey", NewObj("val", -1))

	ctx := NewClientContext()
	conn := &mockConn{}

	cmds := RedisCmds{
		{Cmd: "MULTI", Args: nil},
		{Cmd: "EXPIRE", Args: []string{"ttlkey", "60"}},
		{Cmd: "TTL", Args: []string{"ttlkey"}},
		{Cmd: "EXEC", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)

	obj := Get("ttlkey")
	if obj == nil {
		t.Fatal("expected ttlkey to exist")
	}
	if obj.ExpiresAt == -1 {
		t.Fatal("expected TTL to be set on ttlkey")
	}
}

func TestEmptyEXEC(t *testing.T) {
	setup()
	ctx := NewClientContext()
	conn := &mockConn{}

	cmds := RedisCmds{
		{Cmd: "MULTI", Args: nil},
		{Cmd: "EXEC", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)

	resp := conn.String()
	if !strings.Contains(resp, "*0") {
		t.Fatalf("expected empty array *0, got %q", resp)
	}
	if ctx.InMulti {
		t.Fatal("expected InMulti to be false after empty EXEC")
	}
}

func TestWATCHOnNonExistentKey(t *testing.T) {
	setup()
	ctx := NewClientContext()
	conn := &mockConn{}

	cmds := RedisCmds{
		{Cmd: "WATCH", Args: []string{"ghost"}},
		{Cmd: "MULTI", Args: nil},
		{Cmd: "SET", Args: []string{"ghost", "boo"}},
		{Cmd: "EXEC", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)

	obj := Get("ghost")
	if obj == nil || obj.Value != "boo" {
		t.Fatalf("expected ghost=boo, got %v", obj)
	}
}

func TestWATCHNonExistentThenCreated(t *testing.T) {
	setup()
	ctx := NewClientContext()
	conn := &mockConn{}

	// Watch a key that doesn't exist
	cmds := RedisCmds{
		{Cmd: "WATCH", Args: []string{"ghost"}},
		{Cmd: "MULTI", Args: nil},
		{Cmd: "SET", Args: []string{"ghost", "boo"}},
	}
	EvalAndRespond(cmds, conn, ctx)

	// Another client creates it
	Put("ghost", NewObj("surprise", -1))

	conn.Reset()
	cmds = RedisCmds{
		{Cmd: "EXEC", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)

	if !strings.Contains(conn.String(), "*-1") {
		t.Fatalf("expected WATCH abort (key created externally), got %q", conn.String())
	}
}

func TestContextResetAfterEXEC(t *testing.T) {
	setup()
	ctx := NewClientContext()
	conn := &mockConn{}

	// First transaction
	cmds := RedisCmds{
		{Cmd: "MULTI", Args: nil},
		{Cmd: "SET", Args: []string{"a", "1"}},
		{Cmd: "EXEC", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)

	// Second transaction on same context should work fresh
	conn.Reset()
	cmds = RedisCmds{
		{Cmd: "MULTI", Args: nil},
		{Cmd: "SET", Args: []string{"b", "2"}},
		{Cmd: "EXEC", Args: nil},
	}
	EvalAndRespond(cmds, conn, ctx)

	if Get("a") == nil || Get("b") == nil {
		t.Fatal("both transactions should have succeeded")
	}
}
