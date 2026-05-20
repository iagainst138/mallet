package mallet

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
)

func generateRequests(ctx context.Context, config HttpConfig) (HttpRequests, error) {
	requests := make(HttpRequests)

	loader, err := NewLoader(ctx, config.GeneratorPath, config.Args)
	if err != nil {
		return nil, err
	}

	supportedMethods := []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace,
	}

	go func() {
		defer func() { close(requests) }()
		done := false

		for !done {
			select {
			case <-ctx.Done():
				done = true

			default:
				query, err := loader.Get(ctx)
				if err != nil {
					panic(err)
				}

				if !slices.Contains(supportedMethods, query.Method) {
					panic(fmt.Errorf("unsupported HTTP method %q", query.Method))
				}

				url := config.Addr + query.Query
				req, err := http.NewRequestWithContext(ctx, query.Method, url, query.BodyBytes())
				if err != nil {
					slog.Error("requests", "error", err)
				} else {
					for k, v := range query.Header {
						req.Header.Add(k, v)
					}
					requests <- HttpRequest{Req: req, ParseFunc: ParseJson}
				}
			}
		}
	}()

	return requests, nil
}

func RunHttp(ctx context.Context, config HttpConfig) error {
	requests, err := generateRequests(ctx, config)
	if err != nil {
		return err
	}

	return StartHttp(ctx, config, requests)
}
