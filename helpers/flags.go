package helpers

import (
	"flag"
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

	return nil
}
