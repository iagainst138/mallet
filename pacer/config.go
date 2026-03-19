package pacer

import "time"

type Config struct {
	EPS      int64         `flag:"e" default:"1000"`
	Duration time.Duration `flag:"d" default:"8s"`
	RampUp   time.Duration `flag:"r" default:"2s"`
}

func (c Config) PacerConfig() Config {
	return c
}
