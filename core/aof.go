package core

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"mini-redis/config"
)

var (
	aofFile    *os.File
	aofWriter  *bufio.Writer
	aofMu      sync.Mutex
	aofLoading bool
)

func InitAOF() error {
	if !config.AOFEnabled {
		return nil
	}

	fp, err := os.OpenFile(config.AOFFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	aofFile = fp
	aofWriter = bufio.NewWriter(fp)

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			aofMu.Lock()
			if aofWriter != nil {
				aofWriter.Flush()
			}
			if aofFile != nil {
				aofFile.Sync()
			}
			aofMu.Unlock()
		}
	}()

	log.Println("AOF: persistence enabled, file:", config.AOFFile)
	return nil
}

func LoadAOF() error {
	if !config.AOFEnabled {
		return nil
	}

	data, err := os.ReadFile(config.AOFFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}

	aofLoading = true
	defer func() { aofLoading = false }()

	values, err := Decode(data)
	if err != nil {
		log.Println("AOF: partial decode, replaying what was readable:", err)
	}

	var loaded int
	for _, value := range values {
		array, ok := value.([]interface{})
		if !ok {
			continue
		}
		tokens := make([]string, 0, len(array))
		for _, v := range array {
			s, ok := v.(string)
			if !ok {
				break
			}
			tokens = append(tokens, s)
		}
		if len(tokens) < 2 {
			continue
		}

		switch strings.ToUpper(tokens[0]) {
		case "SET":
			evalSET(tokens[1:])
		case "DEL":
			evalDEL(tokens[1:])
		case "EXPIRE":
			evalEXPIRE(tokens[1:])
		case "LPUSH":
			evalLPUSH(tokens[1:])
		case "RPUSH":
			evalRPUSH(tokens[1:])
		case "LPOP":
			evalLPOP(tokens[1:])
		case "RPOP":
			evalRPOP(tokens[1:])
		case "HSET":
			evalHSET(tokens[1:])
		case "HDEL":
			evalHDEL(tokens[1:])
		}
		loaded++
	}

	log.Printf("AOF: replayed %d commands from %s", loaded, config.AOFFile)
	return nil
}

func WriteAOF(cmd *RedisCmd) {
	if !config.AOFEnabled || aofLoading {
		return
	}

	aofMu.Lock()
	defer aofMu.Unlock()

	if aofWriter == nil {
		return
	}

	tokens := make([]string, 0, 1+len(cmd.Args))
	tokens = append(tokens, cmd.Cmd)
	tokens = append(tokens, cmd.Args...)
	aofWriter.Write(Encode(tokens, false))
}

func RewriteAOF() error {
	if !config.AOFEnabled {
		return nil
	}

	log.Println("AOF: rewrite started")

	tmpPath := config.AOFFile + ".tmp"
	fp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	w := bufio.NewWriter(fp)
	now := time.Now().UnixMilli()

	storeMu.RLock()
	for k, obj := range store {
		if obj.ExpiresAt != -1 && obj.ExpiresAt <= now {
			continue
		}

		switch v := obj.Value.(type) {
		case string:
			tokens := []string{"SET", k, v}
			if obj.ExpiresAt != -1 {
				remainingSec := (obj.ExpiresAt - now) / 1000
				if remainingSec > 0 {
					tokens = append(tokens, "EX", strconv.FormatInt(remainingSec, 10))
				}
			}
			w.Write(Encode(tokens, false))
		case []string:
			if len(v) > 0 {
				tokens := append([]string{"RPUSH", k}, v...)
				w.Write(Encode(tokens, false))
			}
		case map[string]string:
			if len(v) > 0 {
				tokens := []string{"HSET", k}
				for field, val := range v {
					tokens = append(tokens, field, val)
				}
				w.Write(Encode(tokens, false))
			}
		}
	}
	storeMu.RUnlock()

	if err := w.Flush(); err != nil {
		fp.Close()
		os.Remove(tmpPath)
		return err
	}
	fp.Sync()
	fp.Close()

	aofMu.Lock()
	defer aofMu.Unlock()

	if aofWriter != nil {
		aofWriter.Flush()
	}
	if aofFile != nil {
		aofFile.Close()
	}

	if err := os.Rename(tmpPath, config.AOFFile); err != nil {
		return err
	}

	aofFile, err = os.OpenFile(config.AOFFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	aofWriter = bufio.NewWriter(aofFile)

	log.Println("AOF: rewrite complete")
	return nil
}

func CloseAOF() {
	aofMu.Lock()
	defer aofMu.Unlock()

	if aofWriter != nil {
		aofWriter.Flush()
		aofWriter = nil
	}
	if aofFile != nil {
		aofFile.Sync()
		aofFile.Close()
		aofFile = nil
	}
}
