package main

import (
	"crypto/sha256"
)

func stringToSha256(value string) string {
	h := sha256.New()
	h.Write([]byte(value))
	return string(h.Sum(nil))
}
