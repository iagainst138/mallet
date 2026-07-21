package mallet

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"mallet/charting"
	"mallet/pacer"
)

var ErrNotFound = errors.New("Not Found")

func HttpError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

type ParseFunc func(b []byte) error

type HttpRequest struct {
	Req       *http.Request
	ParseFunc ParseFunc
}

func (r HttpRequest) URL() string {
	return r.Req.URL.String()
}

func (r HttpRequest) HasAuthorizationHeader() bool {
	if r.Req == nil {
		return false
	}
	return r.Req.Header.Get("Authorization") != ""
}

func (r HttpRequest) AuthorizationHeader() string {
	if r.Req == nil {
		return ""
	}
	return r.Req.Header.Get("Authorization")
}

type HttpRequests chan HttpRequest

type ApiHttpClient struct {
	ctx context.Context

	config HttpConfig

	pacer      *pacer.Pacer
	recorder   *Recorder
	httpClient *http.Client
}

func NewApiHttpClient(ctx context.Context, config HttpConfig, recorder *Recorder, pacer *pacer.Pacer) (*ApiHttpClient, error) {
	a := &ApiHttpClient{
		ctx: ctx,

		config: config,

		pacer:      pacer,
		recorder:   recorder,
		httpClient: GetHttpClient(config.InsecureSkipVerify),
	}

	return a, nil
}

func (a *ApiHttpClient) Do(req HttpRequest) error {
	var numErrors int

	started := time.Now()
	finished := time.Now()

	ctx, cancel := context.WithTimeout(a.ctx, a.config.HttpTimeout)

	defer func() {
		a.recorder.RecordLatency(started, finished, numErrors)
		cancel()
	}()

	started = time.Now()
	r, err := a.httpClient.Do(req.Req.WithContext(ctx))
	if err != nil {
		finished = time.Now()
		numErrors++
		return err
	}

	body, err := io.ReadAll(r.Body)
	finished = time.Now()
	if err != nil {
		numErrors++
		return err
	}
	defer r.Body.Close()

	if r.StatusCode == http.StatusOK {
		if a.config.ParseResponse && req.ParseFunc != nil {
			if err := req.ParseFunc(body); err != nil {
				slog.Error("failed to parse response", "error", err)
				numErrors++
			}
		}
	}

	if r.StatusCode != http.StatusOK {
		numErrors++
		slog.Error(string(body), "status", r.StatusCode, "url", req.URL(), "auth", req.AuthorizationHeader())
	}

	if a.config.ShowResponse {
		fmt.Fprintf(os.Stderr, "%s\n", body)
	}

	return nil
}

func (a *ApiHttpClient) Worker(requests chan HttpRequest) error {
	done := false

	for !done {
		// TODO review as this could become a blocking point
		req := <-requests

		select {
		case <-a.ctx.Done():
			done = true
		default:
			if err := a.pacer.Get(); err != nil {
				if errors.Is(pacer.PacerCompleted, err) {
					done = true
				} else {
					slog.Error("worker", "error", err)
				}
			} else {
				if err := a.Do(req); err != nil {
					slog.Error("http api client", "error", err)
				}
			}
		}
	}

	return nil
}

func StartHttp(ctx context.Context, config HttpConfig, requests HttpRequests) error {
	if requests == nil {
		return errors.New("HttpRequests channel is nil")
	}

	var wg sync.WaitGroup

	pacer, err := pacer.NewPacer(config.GetPacerConfig())
	if err != nil {
		return err
	}

	recorder, err := NewRecorder(config.GetRecorderConfig(), ApiTableResults, charting.WriteApiChart, Validator(config.PerfConfig))
	if err != nil {
		return err
	}

	wg.Add(config.NumClients)
	for i := 0; i < config.NumClients; i++ {
		c, err := NewApiHttpClient(ctx, config, recorder, pacer)
		if err != nil {
			return err
		}

		go func() { c.Worker(requests); wg.Done() }()
	}

	pacer.Start(ctx)

	wg.Wait()

	summary, err := recorder.Summary()
	fmt.Println(summary)

	return err
}

func GetHttpClient(insecureSkipVerify bool) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			//DialContext: &net.Dialer{
			//	Timeout:   30 * time.Second,
			//	KeepAlive: 30 * time.Second,
			//},
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          1,
			MaxConnsPerHost:       1,
			IdleConnTimeout:       300 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,

			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecureSkipVerify,
			},
		},
	}
}

// SlowlyReadAll will read the first 40 bytes from a reader with a random delay of betwwn 0-9ms
// based on go1.23.3 io.Reader - https://cs.opensource.google/go/go/+/refs/tags/go1.23.3:src/io/io.go;l=709
func SlowlyReadAll(r io.Reader) ([]byte, error) {
	const slowReadLimit = 40

	b := make([]byte, 0, 512)
	bytesRead := 0

	for {
		l := len(b)
		c := cap(b)

		if bytesRead < slowReadLimit {
			c = l + 1
			time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
		}

		n, err := r.Read(b[l:c])
		b = b[:l+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return b, err
		}

		if len(b) == cap(b) {
			// Add more capacity (let append pick how much).
			b = append(b, 0)[:len(b)]
		}

		bytesRead += n
	}
}
