package ghttp

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"helpers"
)

func generateRequests(ctx context.Context, config helpers.HttpConfig) (helpers.HttpRequests, error) {
	requests := make(helpers.HttpRequests)

	loader, err := helpers.NewLoader(ctx, config.GeneratorPath)
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
					requests <- helpers.Request{Req: req, ParseFunc: helpers.ParseJson}
				}
			}
		}
	}()

	return requests, nil
}

func Run(ctx context.Context, config helpers.HttpConfig) error {
	requests, err := generateRequests(ctx, config)
	if err != nil {
		return err
	}

	return helpers.RunHttp(ctx, config, requests)
}
