package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"mallet"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := mallet.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "execution failed: %s\n", err)
		os.Exit(1)
	}
}
