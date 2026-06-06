# Cache DB

A Redis-inspired in-memory key-value database built from scratch in Go with zero external dependencies. Supports the RESP wire protocol, multiple data types, transactions, persistence, and two distinct server architectures.
ompatible
## Features

- **RESP Protocol** — Full encode/decode with command pipelining
- **Data Types** — Strings, Lists, Hashes
- **20+ Commands** — `SET`, `GET`, `DEL`, `TTL`, `EXPIRE`, `PING`, `LPUSH`, `RPUSH`, `LPOP`, `RPOP`, `LRANGE`, `LLEN`, `HSET`, `HGET`, `HDEL`, `HGETALL`, `HLEN`, `HEXISTS`, `BGREWRITEAOF`
- **Transactions** — `MULTI/EXEC/DISCARD/WATCH` with version-based optimistic concurrency control
- **Dual Server Modes** — Async epoll event loop (Linux) and concurrent goroutine-per-connection
- **Key Expiration** — Lazy expiry on read + active expiration via Redis-style adaptive sampling
- **AOF Persistence** — Append-only file with background flush, startup replay, and `BGREWRITEAOF` compaction
- **Docker** — Multi-stage Alpine build, works out of the box

## Quick Start

### Run locally

```bash
go run main.go
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-host` | `0.0.0.0` | Bind address |
| `-port` | `7379` | Listen port |
| `-server-mode` | `auto` | `auto`, `sync`, or `async` |
| `-aof-enabled` | `false` | Enable AOF persistence |
| `-aof-file` | `mini-redis.aof` | AOF file path |
| `-keys-limit` | `5000000` | Max keys before eviction |

### Connect with redis-cli

```bash
redis-cli -p 7379
127.0.0.1:7379> SET hello world
OK
127.0.0.1:7379> GET hello
"world"
127.0.0.1:7379> LPUSH mylist a b c
(integer) 3
127.0.0.1:7379> LRANGE mylist 0 -1
1) "c"
2) "b"
3) "a"
127.0.0.1:7379> HSET user name "alice" age "30"
(integer) 2
127.0.0.1:7379> HGETALL user
1) "name"
2) "alice"
3) "age"
4) "30"
```

### Run with Docker

```bash
docker build -t mini-redis .
docker run -p 7379:7379 mini-redis
```

## Architecture

```
main.go              Entry point, flags, AOF init, server startup
config/config.go     Global configuration
core/
  resp.go            RESP protocol encoder/decoder
  store.go           In-memory store (map + RWMutex + key versioning)
  eval.go            Command dispatcher
  eval_list.go       List commands (LPUSH, RPUSH, LPOP, RPOP, LRANGE, LLEN)
  eval_hash.go       Hash commands (HSET, HGET, HDEL, HGETALL, HLEN, HEXISTS)
  txn.go             MULTI/EXEC/DISCARD/WATCH transactions
  client.go          Per-connection transaction context
  expire.go          Active expiration (adaptive sampling)
  eviction.go        Key eviction at capacity
  aof.go             AOF persistence + rewrite
server/
  async_tcp.go       Epoll event loop (Linux only)
  sync_tcp.go        Goroutine-per-connection (all platforms)
  comm.go            RESP read/decode/respond shared logic
bench/main.go        Benchmark tool (redis-benchmark style)
```

## Benchmarks

Tested inside Docker (Alpine Linux) with **50 concurrent clients** and **100,000 operations** per test.

### Single Request/Response

| Command | Async (epoll) | Sync (goroutines) |
|---------|--------------|-------------------|
| PING | 75,070 ops/s | **89,871 ops/s** |
| SET | 43,162 ops/s | **69,854 ops/s** |
| GET | 35,274 ops/s | **80,812 ops/s** |
| RPUSH | 35,539 ops/s | **45,401 ops/s** |
| LPOP | 42,256 ops/s | **62,564 ops/s** |
| HSET | **47,436 ops/s** | 43,052 ops/s |
| HGET | 33,635 ops/s | **46,552 ops/s** |

### Pipelined (16 cmds/batch)

| Command | Async (epoll) | Sync (goroutines) |
|---------|--------------|-------------------|
| SET | **267,588 ops/s** | 174,715 ops/s |
| GET | **277,683 ops/s** | 169,943 ops/s |
| HSET | 210,375 ops/s | **297,975 ops/s** |

**Key takeaway:** Go's goroutine scheduler is heavily optimized, so the goroutine-per-connection model wins for single request/response. The epoll event loop shines on pipelined workloads where batch processing in a single thread avoids goroutine context-switch overhead.

### Run benchmarks yourself

```bash
# In-process core benchmarks
go test -run=^$ -bench="Benchmark" -benchmem ./core/

# TCP server benchmark (start server first)
go run main.go &
go run bench/main.go -clients 50 -ops 100000

# Docker benchmark (async epoll)
docker build -f Dockerfile.bench -t mini-redis-bench .
docker run -d --name bench mini-redis-bench /mini-redis -server-mode async
docker exec bench /mini-redis-bench -clients 50 -ops 100000
docker rm -f bench
```

## Tests

```bash
go test ./... -v
```

23 tests covering RESP parsing, transaction semantics (MULTI/EXEC/DISCARD/WATCH conflicts, atomicity, context reset), and 19 in-process benchmarks for all data types.
