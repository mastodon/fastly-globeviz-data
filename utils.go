package main

import (
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
		slog.Info(fmt.Sprintf("Got invalid int value for %d, using default value %d", key, defaultValue))
		return defaultValue
	}
	return ret
}
