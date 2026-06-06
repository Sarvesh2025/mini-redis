package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type BenchResult struct {
	Command    string
	TotalOps   int64
	Duration   time.Duration
	OpsPerSec  float64
	AvgLatency time.Duration
	P50        time.Duration
	P99        time.Duration
}

func respEncode(args []string) []byte {
	buf := fmt.Sprintf("*%d\r\n", len(args))
	for _, a := range args {
		buf += fmt.Sprintf("$%d\r\n%s\r\n", len(a), a)
	}
	return []byte(buf)
}

func readResponse(conn net.Conn) error {
	buf := make([]byte, 4096)
	_, err := conn.Read(buf)
	return err
}

func runBenchmark(addr string, clients int, numOps int, cmdName string, cmdBuilder func(i int) []string) BenchResult {
	opsPerClient := numOps / clients
	var totalOps int64
	var wg sync.WaitGroup

	latencies := make([][]time.Duration, clients)
	start := time.Now()

	for c := 0; c < clients; c++ {
		wg.Add(1)
		latencies[c] = make([]time.Duration, 0, opsPerClient)

		go func(clientID int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", addr)
			if err != nil {
				fmt.Printf("  [client %d] connect error: %v\n", clientID, err)
				return
			}
			defer conn.Close()

			baseIdx := clientID * opsPerClient
			for i := 0; i < opsPerClient; i++ {
				cmd := cmdBuilder(baseIdx + i)
				payload := respEncode(cmd)

				opStart := time.Now()
				_, err := conn.Write(payload)
				if err != nil {
					break
				}
				err = readResponse(conn)
				if err != nil {
					break
				}
				latencies[clientID] = append(latencies[clientID], time.Since(opStart))
				atomic.AddInt64(&totalOps, 1)
			}
		}(c)
	}

	wg.Wait()
	duration := time.Since(start)

	allLatencies := make([]time.Duration, 0, numOps)
	for _, cl := range latencies {
		allLatencies = append(allLatencies, cl...)
	}

	var sum time.Duration
	for _, l := range allLatencies {
		sum += l
	}

	var avg, p50, p99 time.Duration
	if len(allLatencies) > 0 {
		avg = sum / time.Duration(len(allLatencies))
		sortDurations(allLatencies)
		p50 = allLatencies[len(allLatencies)*50/100]
		p99 = allLatencies[len(allLatencies)*99/100]
	}

	ops := atomic.LoadInt64(&totalOps)
	return BenchResult{
		Command:    cmdName,
		TotalOps:   ops,
		Duration:   duration,
		OpsPerSec:  float64(ops) / duration.Seconds(),
		AvgLatency: avg,
		P50:        p50,
		P99:        p99,
	}
}

func sortDurations(d []time.Duration) {
	for i := 1; i < len(d); i++ {
		key := d[i]
		j := i - 1
		for j >= 0 && d[j] > key {
			d[j+1] = d[j]
			j--
		}
		d[j+1] = key
	}
}

func runPipelinedBenchmark(addr string, clients int, numOps int, pipelineSize int, cmdName string, cmdBuilder func(i int) []string) BenchResult {
	opsPerClient := numOps / clients
	var totalOps int64
	var wg sync.WaitGroup

	latencies := make([][]time.Duration, clients)
	start := time.Now()

	for c := 0; c < clients; c++ {
		wg.Add(1)
		latencies[c] = make([]time.Duration, 0, opsPerClient/pipelineSize)

		go func(clientID int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", addr)
			if err != nil {
				fmt.Printf("  [client %d] connect error: %v\n", clientID, err)
				return
			}
			defer conn.Close()

			baseIdx := clientID * opsPerClient
			readBuf := make([]byte, 65536)

			for i := 0; i < opsPerClient; i += pipelineSize {
				batch := pipelineSize
				if i+batch > opsPerClient {
					batch = opsPerClient - i
				}

				var payload []byte
				for j := 0; j < batch; j++ {
					cmd := cmdBuilder(baseIdx + i + j)
					payload = append(payload, respEncode(cmd)...)
				}

				opStart := time.Now()
				_, err := conn.Write(payload)
				if err != nil {
					break
				}
				_, err = conn.Read(readBuf)
				if err != nil {
					break
				}
				lat := time.Since(opStart)
				latencies[clientID] = append(latencies[clientID], lat)
				atomic.AddInt64(&totalOps, int64(batch))
			}
		}(c)
	}

	wg.Wait()
	duration := time.Since(start)

	allLatencies := make([]time.Duration, 0, numOps/pipelineSize)
	for _, cl := range latencies {
		allLatencies = append(allLatencies, cl...)
	}

	var sum time.Duration
	for _, l := range allLatencies {
		sum += l
	}

	var avg, p50, p99 time.Duration
	if len(allLatencies) > 0 {
		avg = sum / time.Duration(len(allLatencies))
		sortDurations(allLatencies)
		p50 = allLatencies[len(allLatencies)*50/100]
		p99 = allLatencies[len(allLatencies)*99/100]
	}

	ops := atomic.LoadInt64(&totalOps)
	return BenchResult{
		Command:    cmdName,
		TotalOps:   ops,
		Duration:   duration,
		OpsPerSec:  float64(ops) / duration.Seconds(),
		AvgLatency: avg,
		P50:        p50,
		P99:        p99,
	}
}

func printResult(r BenchResult) {
	fmt.Printf("  %-20s %8.0f ops/sec | avg %8s | p50 %8s | p99 %8s | %d ops in %s\n",
		r.Command,
		r.OpsPerSec,
		r.AvgLatency.Round(time.Microsecond),
		r.P50.Round(time.Microsecond),
		r.P99.Round(time.Microsecond),
		r.TotalOps,
		r.Duration.Round(time.Millisecond),
	)
}

func main() {
	host := flag.String("host", "127.0.0.1", "server host")
	port := flag.Int("port", 7379, "server port")
	clients := flag.Int("clients", 50, "number of concurrent connections")
	numOps := flag.Int("ops", 100000, "total number of operations per test")
	pipeline := flag.Int("pipeline", 16, "pipeline batch size")
	flag.Parse()

	addr := fmt.Sprintf("%s:%d", *host, *port)

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		fmt.Printf("ERROR: cannot connect to %s — is the server running?\n", addr)
		fmt.Println("  Start the server with: go run main.go")
		return
	}
	// Send a real PING to verify server is responding, then reuse the connection
	_, _ = conn.Write(respEncode([]string{"PING"}))
	probe := make([]byte, 64)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Read(probe)
	conn.Close()
	if err != nil {
		fmt.Printf("ERROR: server at %s is not responding\n", addr)
		return
	}

	fmt.Println("=============================================================")
	fmt.Printf(" mini-redis benchmark — %s\n", addr)
	fmt.Printf(" %d clients | %d total ops per test\n", *clients, *numOps)
	fmt.Println("=============================================================")

	fmt.Println()
	fmt.Println("--- Single Request/Response ---")

	results := []BenchResult{}

	r := runBenchmark(addr, *clients, *numOps, "PING", func(i int) []string {
		return []string{"PING"}
	})
	results = append(results, r)
	printResult(r)

	r = runBenchmark(addr, *clients, *numOps, "SET", func(i int) []string {
		return []string{"SET", fmt.Sprintf("key:%d", i), fmt.Sprintf("value-%d", i)}
	})
	results = append(results, r)
	printResult(r)

	r = runBenchmark(addr, *clients, *numOps, "GET", func(i int) []string {
		return []string{"GET", fmt.Sprintf("key:%d", rand.Intn(*numOps))}
	})
	results = append(results, r)
	printResult(r)

	r = runBenchmark(addr, *clients, *numOps, "LPUSH", func(i int) []string {
		return []string{"LPUSH", fmt.Sprintf("bench:list:%d", i%(*clients)), fmt.Sprintf("item-%d", i)}
	})
	results = append(results, r)
	printResult(r)

	r = runBenchmark(addr, *clients, *numOps, "RPUSH", func(i int) []string {
		return []string{"RPUSH", fmt.Sprintf("bench:list2:%d", i%(*clients)), fmt.Sprintf("item-%d", i)}
	})
	results = append(results, r)
	printResult(r)

	r = runBenchmark(addr, *clients, *numOps, "LPOP", func(i int) []string {
		return []string{"LPOP", fmt.Sprintf("bench:list:%d", i%(*clients))}
	})
	results = append(results, r)
	printResult(r)

	r = runBenchmark(addr, *clients, *numOps, "HSET", func(i int) []string {
		return []string{"HSET", "bench:hash", fmt.Sprintf("field-%d", i), fmt.Sprintf("val-%d", i)}
	})
	results = append(results, r)
	printResult(r)

	r = runBenchmark(addr, *clients, *numOps, "HGET", func(i int) []string {
		return []string{"HGET", "bench:hash", fmt.Sprintf("field-%d", rand.Intn(*numOps))}
	})
	results = append(results, r)
	printResult(r)

	fmt.Println()
	fmt.Printf("--- Pipelined (%d cmds/batch) ---\n", *pipeline)

	r = runPipelinedBenchmark(addr, *clients, *numOps, *pipeline, "SET (pipelined)", func(i int) []string {
		return []string{"SET", fmt.Sprintf("pkey:%d", i), fmt.Sprintf("pval-%d", i)}
	})
	results = append(results, r)
	printResult(r)

	r = runPipelinedBenchmark(addr, *clients, *numOps, *pipeline, "GET (pipelined)", func(i int) []string {
		return []string{"GET", fmt.Sprintf("pkey:%d", rand.Intn(*numOps))}
	})
	results = append(results, r)
	printResult(r)

	r = runPipelinedBenchmark(addr, *clients, *numOps, *pipeline, "LPUSH (pipelined)", func(i int) []string {
		return []string{"LPUSH", fmt.Sprintf("bench:plist:%d", i%(*clients)), fmt.Sprintf("item-%d", i)}
	})
	results = append(results, r)
	printResult(r)

	r = runPipelinedBenchmark(addr, *clients, *numOps, *pipeline, "HSET (pipelined)", func(i int) []string {
		return []string{"HSET", "bench:phash", fmt.Sprintf("f-%d", i), fmt.Sprintf("v-%d", i)}
	})
	results = append(results, r)
	printResult(r)

	fmt.Println()
	fmt.Println("=============================================================")
	fmt.Println(" Summary")
	fmt.Println("=============================================================")
	for _, r := range results {
		fmt.Printf("  %-20s → %10.0f ops/sec\n", r.Command, r.OpsPerSec)
	}
	fmt.Println("=============================================================")
}
