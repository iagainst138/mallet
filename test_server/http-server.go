package tserver

import (
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"regexp"
	"time"
)

var (
	okString    = []byte("OK\n")
	readyString = []byte("ready\n")
)

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

func handleRequestWithBody(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	defer r.Body.Close()

	if len(b) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	} else {
		w.Write(okString)
	}

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

	http.HandleFunc("GET /test/", h.responseWrapper(okString))
	http.HandleFunc("GET /args/", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if len(r.Form) == 0 {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		} else {
			w.Write(okString)
		}
	})
	http.HandleFunc("GET /headers/", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if len(r.Form) == 0 {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		header := r.Form.Get("header")
		value := r.Form.Get("value")

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
	http.HandleFunc("POST /post-test/", handleRequestWithBody)
	http.HandleFunc("PUT /put-test/", handleRequestWithBody)

	slog.Info("http server", "addr", listenAddr, "delay", delay)

	return http.ListenAndServeTLS(listenAddr, "certs/cert.pem", "certs/key.pem", nil)
}
