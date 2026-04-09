package tserver

import (
	"log/slog"
	"math/rand"
	"net/http"
	"regexp"
	"time"
)

var readyString = []byte("ready\n")

type httpServer struct {
	enableErrors bool
	delay        int64
	authRegex    *regexp.Regexp
}

func (h *httpServer) slowDown() {
	if h.delay > 0 {
		time.Sleep(time.Duration(rand.Int63n(h.delay)))
	}
}

func (h *httpServer) shouldError() bool {
	return h.enableErrors && rand.Intn(100) < 10
}

func (h *httpServer) wrap(f http.HandlerFunc) http.HandlerFunc {
	w := func(w http.ResponseWriter, r *http.Request) {
		h.slowDown()

		//authHeader := r.Header.Get("Authorization")
		//if !h.authRegex.MatchString(authHeader) {
		//	helpers.HttpError(w, http.StatusUnauthorized)
		//	return
		//}

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

func HttpServer(listenAddr string, enableErrors bool, delay time.Duration) error {
	h := httpServer{
		enableErrors: enableErrors,
		delay:        delay.Nanoseconds(),
		authRegex:    regexp.MustCompile("^[a-zA-Z]{1,20} .*"),
	}

	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Write(readyString)
	})

	http.HandleFunc("/test/", h.responseWrapper([]byte("{}")))

	slog.Info("http server", "addr", listenAddr, "delay", delay)

	return http.ListenAndServeTLS(listenAddr, "certs/cert.pem", "certs/key.pem", nil)
}
