# Build
FROM golang:1.22-alpine AS builder
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /mini-redis .

# Run
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /

COPY --from=builder /mini-redis /mini-redis

EXPOSE 7379

ENTRYPOINT ["/mini-redis", "-host", "0.0.0.0", "-port", "7379"]
