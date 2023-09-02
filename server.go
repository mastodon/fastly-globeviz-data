package main

import (
	"bufio"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

var sseChannel chan string

var port = getIntEnv("PORT", 4000)
var debug = false
var maxStreamDuration = 30 // in seconds
var pingInterval = 1       // in seconds
var retryDuration = 1      // in seconds

func streamEventsHandler(w http.ResponseWriter, r *http.Request) {
	slog.Debug("New connection established")

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)

	if !ok {
		panic("Unable to initialize flusher")
	}

	// Send the retry info after the connection opens
	fmt.Fprintf(w, "retry: %d\n\n", retryDuration*1000)
	flusher.Flush()

	// This is used to force-close the connection after some time, clients will reconnect and it avoids long-running connections
	timeout := time.After(time.Duration(maxStreamDuration) * time.Second)

	// Send an empty message every `pingInterval` seconds
	var pingTickerChan <-chan time.Time
	if pingInterval != 0 {
		pingTicker := time.NewTicker(time.Duration(pingInterval) * time.Second)
		pingTickerChan = pingTicker.C

		defer pingTicker.Stop()

	}

	id := 0

	for {
		select {
		case msg := <-sseChannel:
			fmt.Fprintf(w, "id: %d\nevent: traffic-log\ndata: %s\n\n", id, msg)
			id++
			flusher.Flush()
		case <-pingTickerChan:
			fmt.Fprintf(w, "data:\n\n")
			flusher.Flush()
		case <-timeout:
			slog.Debug("Connection closed after timeout")
			return

		case <-r.Context().Done():
			slog.Debug("Connection closed")
			return
		}
	}
}

func sendEventsHandler(w http.ResponseWriter, r *http.Request) {
	if sseChannel == nil {
		panic("Channel not initialized")
	}

	scanner := bufio.NewScanner(r.Body)

	for scanner.Scan() {
		text := scanner.Text()
		slog.Debug("Received message", "message", text)
		select {
		case sseChannel <- text:
		default:
			slog.Debug("No receivers, dropping message")
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error(err.Error())
	}

	slog.Info("Written")

	w.WriteHeader(http.StatusOK)
}

func main() {
	// CLI flags
	flag.IntVar(&port, "port", port, "port to listen on")
	flag.IntVar(&pingInterval, "ping-interval", pingInterval, "interval between ping messages (in seconds, 0 to disable)")
	flag.IntVar(&maxStreamDuration, "max-stream-duration", maxStreamDuration, "maximum duration for streaming connections (in seconds)")
	flag.IntVar(&retryDuration, "retry-duration", retryDuration, "retry duration to advertise to clients (in seconds)")
	flag.BoolVar(&debug, "debug", debug, "enable debug logging")

	flag.Parse()

	// Setup logging
	var programLevel = new(slog.LevelVar) // Info by default

	logHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: programLevel})
	slog.SetDefault(slog.New(logHandler))

	if debug {
		programLevel.Set(slog.LevelDebug)
	}

	router := http.NewServeMux()

	sseChannel = make(chan string)

	defer func() {
		close(sseChannel)
		sseChannel = nil
	}()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			switch r.Method {
			case "GET":
				streamEventsHandler(w, r)
			case "POST":
				sendEventsHandler(w, r)
			}
		} else {
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	})

	slog.Info(fmt.Sprintf("Server started on port %d", port))

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), router); err != nil {
		slog.Error(err.Error())
	}
}
