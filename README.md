# mini-redis

Starter Go layout similar to [DiceDB](https://github.com/DiceDB/dice): root `main.go`, `config/`, `server/`, and `bin/` for local builds.

## Run

```bash
go run .
```

Flags (defaults match the usual pattern):

- `-host` — default `0.0.0.0`
- `-port` — default `7379`

Example:

```bash
go run . -host 127.0.0.1 -port 6379
```

The server echoes bytes back over TCP (placeholder until you add a Redis-style protocol).

## Build into `bin`

```bash
go build -o bin/mini-redis .
```
