package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
)

func sendDataToProxy(data []string) {
	if forwardRequestsTo == "" {
		return
	}

	req, err := http.NewRequest("POST", forwardRequestsTo, bytes.NewReader([]byte(strings.Join(data, "\n"))))

	if err != nil {
		slog.Error((err.Error()))
		return
	}

	req.Header.Add("Content-Type", "text/plain")

	resp, err := httpClient.Do(req)

	if err != nil {
		slog.Error(err.Error())
		return
	}

	defer resp.Body.Close()
}
