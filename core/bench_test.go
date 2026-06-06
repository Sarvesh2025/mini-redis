package core

import (
	"fmt"
	"strconv"
	"testing"
)

func resetStore() {
	storeMu.Lock()
	store = make(map[string]*Obj)
	keyVersions = make(map[string]uint64)
	storeMu.Unlock()
}

func BenchmarkSET(b *testing.B) {
	resetStore()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		evalSET([]string{fmt.Sprintf("key:%d", i), fmt.Sprintf("value-%d", i)})
	}
}

func BenchmarkGET(b *testing.B) {
	resetStore()
	for i := 0; i < 10000; i++ {
		evalSET([]string{fmt.Sprintf("key:%d", i), fmt.Sprintf("value-%d", i)})
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		evalGET([]string{fmt.Sprintf("key:%d", i%10000)})
	}
}

func BenchmarkSETWithExpiry(b *testing.B) {
	resetStore()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		evalSET([]string{fmt.Sprintf("key:%d", i), "value", "EX", "3600"})
	}
}

func BenchmarkDEL(b *testing.B) {
	resetStore()
	for i := 0; i < b.N; i++ {
		evalSET([]string{fmt.Sprintf("key:%d", i), "value"})
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		evalDEL([]string{fmt.Sprintf("key:%d", i)})
	}
}

func BenchmarkLPUSH(b *testing.B) {
	resetStore()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		evalLPUSH([]string{"bench:list", fmt.Sprintf("item-%d", i)})
	}
}

func BenchmarkRPUSH(b *testing.B) {
	resetStore()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		evalRPUSH([]string{"bench:list", fmt.Sprintf("item-%d", i)})
	}
}

func BenchmarkLPOP(b *testing.B) {
	resetStore()
	for i := 0; i < b.N; i++ {
		evalRPUSH([]string{"bench:list", "item"})
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		evalLPOP([]string{"bench:list"})
	}
}

func BenchmarkRPOP(b *testing.B) {
	resetStore()
	for i := 0; i < b.N; i++ {
		evalRPUSH([]string{"bench:list", "item"})
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		evalRPOP([]string{"bench:list"})
	}
}

func BenchmarkLRANGE_100(b *testing.B) {
	resetStore()
	for i := 0; i < 100; i++ {
		evalRPUSH([]string{"bench:list", fmt.Sprintf("item-%d", i)})
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		evalLRANGE([]string{"bench:list", "0", "99"})
	}
}

func BenchmarkHSET(b *testing.B) {
	resetStore()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		evalHSET([]string{"bench:hash", fmt.Sprintf("field-%d", i), fmt.Sprintf("value-%d", i)})
	}
}

func BenchmarkHGET(b *testing.B) {
	resetStore()
	for i := 0; i < 10000; i++ {
		evalHSET([]string{"bench:hash", fmt.Sprintf("field-%d", i), fmt.Sprintf("value-%d", i)})
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		evalHGET([]string{"bench:hash", fmt.Sprintf("field-%d", i%10000)})
	}
}

func BenchmarkHDEL(b *testing.B) {
	resetStore()
	for i := 0; i < b.N; i++ {
		evalHSET([]string{"bench:hash", fmt.Sprintf("field-%d", i), "val"})
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		evalHDEL([]string{"bench:hash", fmt.Sprintf("field-%d", i)})
	}
}

func BenchmarkHGETALL_100(b *testing.B) {
	resetStore()
	for i := 0; i < 100; i++ {
		evalHSET([]string{"bench:hash", fmt.Sprintf("field-%d", i), fmt.Sprintf("value-%d", i)})
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		evalHGETALL([]string{"bench:hash"})
	}
}

func BenchmarkParallelSET(b *testing.B) {
	resetStore()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			evalSET([]string{"pkey:" + strconv.Itoa(i), "value"})
			i++
		}
	})
}

func BenchmarkParallelGET(b *testing.B) {
	resetStore()
	for i := 0; i < 10000; i++ {
		evalSET([]string{fmt.Sprintf("key:%d", i), "value"})
	}
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			evalGET([]string{fmt.Sprintf("key:%d", i%10000)})
			i++
		}
	})
}

func BenchmarkParallelHSET(b *testing.B) {
	resetStore()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			evalHSET([]string{"bench:hash", "field-" + strconv.Itoa(i), "val"})
			i++
		}
	})
}

func BenchmarkMULTIEXEC_3cmds(b *testing.B) {
	resetStore()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := NewClientContext()
		evalMULTI(ctx)
		ctx.QueueCmd(&RedisCmd{Cmd: "SET", Args: []string{"tx-key", "tx-val"}})
		ctx.QueueCmd(&RedisCmd{Cmd: "LPUSH", Args: []string{"tx-list", "item"}})
		ctx.QueueCmd(&RedisCmd{Cmd: "HSET", Args: []string{"tx-hash", "f1", "v1"}})
		evalEXEC(ctx)
	}
}

func BenchmarkRESPEncode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Encode("hello world this is a benchmark value", false)
	}
}

func BenchmarkRESPDecode(b *testing.B) {
	data := []byte("*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$7\r\nmyvalue\r\n")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		Decode(data)
	}
}
