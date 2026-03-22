package helpers

import (
	"context"
	"fmt"
	"log"
	"net/http"
)

func generateRequests(ctx context.Context, config HttpConfig) (HttpRequests, error) {
	requests := make(HttpRequests)

	loader, err := NewLoader(ctx, config.GeneratorPath)
	if err != nil {
		return requests, err
	}

	go func() {
		done := false

		for !done {
			select {
			case <-ctx.Done():
				done = true
				break

			default:
				query, err := loader.Get(ctx)
				if err != nil {
					panic(err)
				}

				switch query.Method {
				case http.MethodGet:
					// ok
				default:
					panic(fmt.Errorf("unsupported HTTP method %q", query.Method))
				}

				url := config.Addr + query.Query
				req, err := http.NewRequestWithContext(ctx, query.Method, url, nil)
				if err != nil {
					log.Println(err)
				} else {
					requests <- Request{Req: req, ParseFunc: ParseJson}
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
