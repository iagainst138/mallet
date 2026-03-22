package pacer

import (
	"context"
	"errors"
	"log"
	"math"
	"sync"
	"time"
)

const (
	numIntervals     = 8
	intervalDuration = time.Second / time.Duration(numIntervals)
	timeFormat       = "15:04:05.000"
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

	started := time.Now()
	rampUpEnd := started.Add(p.RampUp)
	end := started.Add(p.Duration)

	var leftOver int64

	ctx, cancel := context.WithDeadline(ctx, end)
	defer cancel()

	//defer func() { fmt.Printf("completed in %s\n", time.Since(started)) }() // debug

	for {
		loopStart := time.Now()
		loopEnd := loopStart.Add(time.Second)
		eps := p.EPS

		//fmt.Println(loopStart.Format(timeFormat), loopEnd.Format(timeFormat)) // debug

		if loopStart.Before(rampUpEnd) {
			percentage := time.Since(started).Seconds() / p.RampUp.Seconds()
			eps = int64(math.Ceil(float64(p.EPS) * percentage))
		}

		var totalSleep time.Duration
		var segmentEPS int64

		for i := 1; i <= numIntervals; i++ {
			segmentStart := time.Now()
			segmentEnd := segmentStart.Add(intervalDuration)
			if !segmentEnd.Before(loopEnd) {
				segmentEnd = loopEnd
			}

			select {
			case <-ctx.Done():
				//fmt.Printf(" * %d %d\n", i, segmentEPS) // debug
				return ctx.Err()
			default:
				// pass through
			}

			intervalEPS := (eps + leftOver) / numIntervals

			modulo := eps % numIntervals
			if i <= int(modulo) {
				intervalEPS += 1
			}

			sendContext, sendCancel := context.WithDeadline(ctx, segmentEnd)
			leftOver = p.do(sendContext, intervalEPS)
			sendCancel()

			sleepDuration := segmentEnd.Sub(time.Now())
			time.Sleep(sleepDuration)

			totalSleep += sleepDuration
			segmentEPS += intervalEPS
		}

		//fmt.Println(" +", segmentEPS) // debug

	}
}

func (p *Pacer) Validate() bool {
	ok := true

	if p.EPS < 1 {
		log.Printf("error: Pacer.EPS < 1:%d", p.EPS)
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

func (p *Pacer) do(ctx context.Context, n int64) int64 {
	var numSent int64

	for i := 0; i < int(n); i++ {
		select {
		case <-ctx.Done():
			return n - numSent
		case p.ch <- 0:
			numSent++
			break
		}
	}

	return n - numSent
}
