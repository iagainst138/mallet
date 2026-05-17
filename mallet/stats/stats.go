package stats

import (
	"os"
	"time"

	"github.com/go-go-golems/bobatea/pkg/sparkline"

	"mallet/term"
)

var (
	sparkLine *sparkline.Sparkline

	lastUpdate time.Time
)

func Update(latency time.Duration, p95 time.Duration) {
	if time.Since(lastUpdate) > term.WriteInterval {
		sparkLine.AddPoint(float64(latency.Nanoseconds()))
		lastUpdate = time.Now()
	}

	term.Fprintf(os.Stderr, "[%s] | latency:%s | p95:%s",
		sparkLine.Render(),
		latency,
		p95,
	)
}

func init() {
	points := 20

	config := sparkline.Config{
		Width:     points,
		Height:    6,
		MaxPoints: points,
		Style:     sparkline.StyleBars,
		//Title:     "Network Traffic (MB/s)",
		//ShowValue: true,
	}
	sparkLine = sparkline.New(config)
}
