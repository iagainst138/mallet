package pacer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
)

const (
	numIntervals     = 8
	intervalDuration = time.Second / time.Duration(numIntervals)
	timeFormat       = "15:04:05.000"

	// maxSegmentSlack is the maximum grace period allowed after a segment's nominal
	// end for workers to pick up tokens that were already scheduled, preventing late
	// tokens from being dropped when workers experience momentary scheduling delays.
	maxSegmentSlack = 20 * time.Millisecond

	// minSpacingWait is the shortest wait do sleeps for. Below it a timer costs more
	// than the wait is worth - at high rates the gap between tokens is a few
	// microseconds - so the token goes out at once; the schedule being absolute,
	// that does not change the rate, only how finely it is spread.
	minSpacingWait = 50 * time.Microsecond
)

var PacerCompleted = errors.New("pacer completed")

type GetFunc func() error

type PacerConfig struct {
	EPS      int64         `flag:"e" default:"1000"`
	Duration time.Duration `flag:"d" default:"8s"`
	RampUp   time.Duration `flag:"r" default:"2s"`
}

func (c PacerConfig) GetPacerConfig() PacerConfig {
	return c
}

type Pacer struct {
	sync.Mutex

	EPS      int64
	Duration time.Duration
	RampUp   time.Duration

	loopError error
	wg        sync.WaitGroup
	ch        chan int

	// started is used to calculate how long is left
	started time.Time
}

func NewPacer(config PacerConfig) (*Pacer, error) {
	p := &Pacer{
		loopError: nil,
		ch:        make(chan int, 0),
		wg:        sync.WaitGroup{},

		EPS:      config.EPS,
		Duration: config.Duration,
		RampUp:   config.RampUp,
	}

	return p, nil
}

func (p *Pacer) Start(ctx context.Context) error {
	p.wg.Add(1)

	go func() {
		p.loopError = p.loop(ctx)
		close(p.ch)
		p.wg.Done()
	}()

	return nil
}

func (p *Pacer) Get() error {
	if _, ok := <-p.ch; !ok {
		return PacerCompleted
	}

	return nil
}

func (p *Pacer) loop(ctx context.Context) error {
	if !p.Validate() {
		return errors.New("validation failed, refer to logs")
	}

	p.started = time.Now()
	rampUpEnd := p.started.Add(p.RampUp)
	runEnd := p.started.Add(p.Duration)
	end := runEnd.Add(maxSegmentSlack)

	var leftOver int64

	ctx, cancel := context.WithDeadline(ctx, end)
	defer cancel()

	for sec := 0; ; sec++ {
		loopStart := p.started.Add(time.Duration(sec) * time.Second)
		if !loopStart.Before(runEnd) {
			<-ctx.Done()
			return ctx.Err()
		}
		loopEnd := loopStart.Add(time.Second)
		eps := p.EPS

		if loopStart.Before(rampUpEnd) {
			percentage := time.Since(p.started).Seconds() / p.RampUp.Seconds()
			eps = int64(math.Ceil(float64(p.EPS) * percentage))
		}

		var totalSleep time.Duration
		var segmentEPS int64

		for i := 1; i <= numIntervals; i++ {
			segmentEnd := loopStart.Add(time.Duration(i) * intervalDuration)
			if !segmentEnd.Before(loopEnd) {
				segmentEnd = loopEnd
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				// pass through
			}

			intervalEPS := eps / numIntervals

			modulo := eps % numIntervals
			if i <= int(modulo) {
				intervalEPS += 1
			}

			// tokens the last segment did not get to are owed in full
			intervalEPS += leftOver

			sendContext, sendCancel := context.WithDeadline(ctx, segmentEnd.Add(maxSegmentSlack))
			leftOver = p.do(sendContext, intervalEPS)
			sendCancel()

			sleepDuration := segmentEnd.Sub(time.Now())
			if sleepDuration > 0 {
				time.Sleep(sleepDuration)
			}

			totalSleep += sleepDuration
			segmentEPS += intervalEPS
		}
	}
}

func (p *Pacer) Validate() bool {
	ok := true

	if p.EPS < 1 {
		slog.Error("pacer", "error", fmt.Sprintf("bad EPS:%d", p.EPS))
		ok = false
	}

	return ok
}

func (p *Pacer) Err() error {
	return p.loopError
}

func (p *Pacer) Wait() error {
	p.wg.Wait()

	return p.loopError
}

// do hands out n tokens, spread evenly over the time left before ctx's
// deadline and returns how many it did not get to.
//
// Token i is due at start + i*spacing. One that is late - every worker was busy
// - goes out as soon as one is free, and the tokens after it keep to the same
// schedule, so falling behind is caught up without slowing the rate.
func (p *Pacer) do(ctx context.Context, n int64) int64 {
	if n <= 0 {
		return 0
	}

	start := time.Now()
	deadline, hasDeadline := ctx.Deadline()
	var spacing time.Duration
	if hasDeadline {
		window := deadline.Sub(start)
		if window <= 0 {
			return n
		}
		if window > intervalDuration {
			spacing = intervalDuration / time.Duration(n)
		} else {
			spacing = window / time.Duration(n)
		}
	} else {
		spacing = intervalDuration / time.Duration(n)
	}

	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()

	var numSent int64

	for i := int64(0); i < n; i++ {
		if wait := time.Until(start.Add(time.Duration(i) * spacing)); wait >= minSpacingWait {
			timer.Reset(wait)
			select {
			case <-ctx.Done():
				return n - numSent
			case <-timer.C:
			}
		}

		// The deadline is checked here rather than left to ctx.Done(),
		// which closes when its timer fires - a little after the deadline.
		// In that gap a segment that started too late, past the end of
		// the run, would hand out every token it had at once.
		if hasDeadline && !time.Now().Before(deadline) {
			return n - numSent
		}

		if ctx.Err() != nil {
			return n - numSent
		}

		select {
		case <-ctx.Done():
			return n - numSent
		case p.ch <- 0:
			numSent++
		}
	}

	return n - numSent
}

func (p *Pacer) PercentageComplete() float64 {
	end := p.started.Add(p.Duration)
	currentDuration := end.Sub(time.Now())

	r := 100.0 - ((float64(currentDuration.Nanoseconds()) / float64(p.Duration.Nanoseconds())) * 100)

	if r > 100 {
		r = 100
	}

	return r
}
