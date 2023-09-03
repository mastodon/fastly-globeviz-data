package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"
)

var (
	port              = "4000"
	debug             = false
	maxStreamDuration = 30 // in seconds
	pingInterval      = 1  // in seconds
	retryDuration     = 1  // in seconds
	fastlyServiceName = "*"
)

func streamEventsHandler(w http.ResponseWriter, r *http.Request, b *Broker[string]) {
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
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(maxStreamDuration)*time.Second)
	defer cancel()

	// Send an empty message every `pingInterval` seconds
	var pingTickerChan <-chan time.Time

	if pingInterval != 0 {
		pingTicker := time.NewTicker(time.Duration(pingInterval) * time.Second)
		pingTickerChan = pingTicker.C

		defer pingTicker.Stop()
	}

	brokerChannel := b.Subscribe()

	for {
		select {
		case msg := <-brokerChannel:
			fmt.Fprintf(w, "event: traffic-log\ndata: %s\n\n", msg)
			flusher.Flush()
		case <-pingTickerChan:
			fmt.Fprintf(w, "data:\n\n")
			flusher.Flush()
		case <-ctx.Done():
			slog.Debug("Connection closed")
			b.Unsubscribe(brokerChannel)
			return
		}
	}
}

func sendEventsHandler(w http.ResponseWriter, r *http.Request, b *Broker[string]) {
	scanner := bufio.NewScanner(r.Body)

	for scanner.Scan() {
		text := scanner.Text()
		slog.Debug("Received message", "message", text)
		b.Publish(text)
	}

	if err := scanner.Err(); err != nil {
		slog.Error(err.Error())
	}

	w.WriteHeader(http.StatusOK)
}

func main() {
	// CLI flags
	flag.StringVar(&fastlyServiceName, "fastly-service-name", fastlyServiceName, "Fastly service name to accept logs from")
	flag.StringVar(&port, "port", "4000", "port to listen on")
	flag.IntVar(&pingInterval, "ping-interval", pingInterval, "interval between ping messages (in seconds, 0 to disable)")
	flag.IntVar(&maxStreamDuration, "max-stream-duration", maxStreamDuration, "maximum duration for streaming connections (in seconds)")
	flag.IntVar(&retryDuration, "retry-duration", retryDuration, "retry duration to advertise to clients (in seconds)")
	flag.BoolVar(&debug, "debug", debug, "enable debug logging")

	flag.Parse()

	// Setup logging
	var programLevel slog.LevelVar // Info by default

	logHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: &programLevel})
	slog.SetDefault(slog.New(logHandler))

	if debug {
		programLevel.Set(slog.LevelDebug)
	}

	b := NewBroker[string]()
	go b.Start()

	router := http.NewServeMux()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			switch r.Method {
			case "GET":
				streamEventsHandler(w, r, b)
			case "POST":
				sendEventsHandler(w, r, b)
			case "OPTIONS":
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
		} else {
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	})

	// Allow this tool to be a log destination for Fastly's real time logging feature
	// https://developer.fastly.com/learning/integrations/logging/#http-challenge
	router.HandleFunc("/.well-known/fastly/logging/challenge", func(w http.ResponseWriter, r *http.Request) {
		if fastlyServiceName == "*" {
			fmt.Fprintf(w, "*")
		} else {
			svcSum := stringToSha256(fastlyServiceName)
			fmt.Fprintf(w, "%s", svcSum)
		}
	})

	slog.Info(fmt.Sprintf("Server started on port %s", port))

	if err := http.ListenAndServe(net.JoinHostPort("", port), router); err != nil {
		slog.Error(err.Error())
	}
}
