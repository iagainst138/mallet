package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"loadgen"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := loadgen.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "execution failed: %s\n", err)
		os.Exit(1)
	}
}
