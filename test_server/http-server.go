package tserver

import (
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"
)

var (
	okString    = []byte("OK\n")
	readyString = []byte("ready\n")
)

type httpServer struct {
	enableErrors bool
	delay        int64
}

func (h *httpServer) slowDown() {
	if h.delay > 0 {
		time.Sleep(time.Duration(rand.Int64N(h.delay)))
	}
}

func (h *httpServer) shouldError() bool {
	return h.enableErrors && rand.IntN(100) < 10
}

func (h *httpServer) wrap(f http.HandlerFunc) http.HandlerFunc {
	w := func(w http.ResponseWriter, r *http.Request) {
		h.slowDown()

		if h.shouldError() {
			http.Error(w, "injected failure", 599)
		} else {
			f(w, r)
		}
	}

	return w
}

func (h *httpServer) responseWrapper(b []byte) http.HandlerFunc {
	f := func(w http.ResponseWriter, r *http.Request) {
		w.Write(b)
	}

	return h.wrap(f)
}

func handleRequestWithBody(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var buf [1]byte
	n, err := r.Body.Read(buf[:])
	if n == 0 && (err == io.EOF || err != nil) {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	_, _ = io.Copy(io.Discard, r.Body)
	w.Write(okString)
}

func HttpServer(listenAddr string, enableErrors bool, delay time.Duration) error {
	h := httpServer{
		enableErrors: enableErrors,
		delay:        delay.Nanoseconds(),
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Write(readyString)
	})

	mux.HandleFunc("GET /test/", h.responseWrapper(okString))
	mux.HandleFunc("GET /with-auth/", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		w.Write(okString)
	})
	mux.HandleFunc("GET /args/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery == "" || len(r.URL.Query()) == 0 {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		w.Write(okString)
	})
	mux.HandleFunc("GET /headers/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if len(q) == 0 {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		header := q.Get("header")
		value := q.Get("value")

		if len(header) == 0 || len(value) == 0 {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if r.Header.Get(header) != value {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		w.Write(okString)
	})
	mux.HandleFunc("POST /post-test/", handleRequestWithBody)
	mux.HandleFunc("PUT /put-test/", handleRequestWithBody)

	slog.Info("http server", "addr", listenAddr, "delay", delay)

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return srv.ListenAndServeTLS("certs/cert.pem", "certs/key.pem")
}
