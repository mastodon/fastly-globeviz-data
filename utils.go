package main

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

func getIntEnv(key string, defaultValue int) int {
	val := os.Getenv(key)

	if val == "" {
		return defaultValue
	}

	ret, err := strconv.Atoi(val)
	if err != nil {
		slog.Info(fmt.Sprintf("Got invalid int value for %s, using default value %d", key, defaultValue))
		return defaultValue
	}
	return ret
}

func stringToSha256(value string) string {
	h := sha256.New()
	h.Write([]byte(value))
	return string(h.Sum(nil))
}
