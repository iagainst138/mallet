package mallet

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"maps"
	"net/http"
	_ "net/http/pprof"
	"os"
	"slices"
)

type runFunc func(ctx context.Context, cmd string) error

type commandMap map[string]runFunc

func (c commandMap) supportedCommands() []string {
	supportedCommands := make([]string, len(c))
	i := 0
	for k := range maps.Keys(c) {
		supportedCommands[i] = k
		i++
	}
	slices.Sort(supportedCommands)

	return supportedCommands
}

func runHttp(ctx context.Context, cmd string) error {
	config := GetHttpConfig()

	if err := ParseFlags(cmd, &config); err != nil {
		return err
	}

	return RunHttp(ctx, config)
}

func Run(ctx context.Context) error {
	m := commandMap{
		"http": runHttp,
	}

	if len(os.Args) < 2 {
		return fmt.Errorf("insufficient args - expected >= 2 got %d, please specify a command from %v", len(os.Args), m.supportedCommands())
	}

	cmd := os.Args[1]

	f, exists := m[cmd]
	if !exists {
		return fmt.Errorf("unknown command %q, supported commands are %v", cmd, m.supportedCommands())
	}

	if os.Getenv("ENABLE_PROFILING") == "1" {
		log.Println("Enabling pprof for profiling")
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	slog.Info("starting", "command", cmd)
	return f(ctx, cmd)
}
