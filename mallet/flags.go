package mallet

import (
	"flag"
	"log/slog"
	"os"

	"github.com/itzg/go-flagsfiller"
)

func ParseFlags(cmd string, config any) error {
	f := flag.NewFlagSet(cmd, flag.ExitOnError)

	filler := flagsfiller.New()
	if err := filler.Fill(f, config); err != nil {
		return err
	}

	f.Parse(os.Args[2:])

	if logFile := f.Lookup("logfile"); logFile != nil {
		if logFile.Value.String() != "" {
			logger, _, err := SetupDualLogger(logFile.Value.String())
			if err != nil {
				return err
			}
			slog.SetDefault(logger)
		}
	}

	return nil
}
