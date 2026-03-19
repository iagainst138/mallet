package pacer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func isContextError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func TestDeadline(t *testing.T) {
	const bufferTime = 2000 * time.Millisecond
	const numWorkers = 120
	const maxOverflowPercentage = 10.0

	testCases := []struct {
		eps           int64
		pacerDuration time.Duration
		long          bool
	}{
		{eps: 160, pacerDuration: time.Second * 2},
		{eps: 800, pacerDuration: time.Second * 2},
		{eps: 3200, pacerDuration: time.Second * 2},
		{eps: 150, pacerDuration: time.Second * 2},
		{eps: 333, pacerDuration: time.Second * 2},
		{eps: 60000, pacerDuration: time.Second * 2},

		// TODO add short flag
		{eps: 60000, pacerDuration: time.Second * 60, long: true},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d eps / %s", tc.eps, tc.pacerDuration), func(t *testing.T) {
			if testing.Short() && tc.long {
				return
			}

			var expected int64 = int64(tc.pacerDuration.Seconds()) * tc.eps
			var wg sync.WaitGroup
			var counter atomic.Int64

			passDuration := tc.pacerDuration + bufferTime

			p, err := NewPacer(Config{Duration: tc.pacerDuration, EPS: tc.eps})
			if err != nil {
				t.Fatal(err)
			}

			started := time.Now()
			err = p.Start(context.TODO())
			if err != nil {
				t.Fatal(err)
			}

			f := func() {
				var x time.Duration = 1000
				for {
					if err := p.Get(); err != nil {
						break
					}
					time.Sleep(x * time.Microsecond)
					if x > 0 {
						x--
					}
					counter.Add(1)
				}
			}

			wg.Add(numWorkers)
			for i := 0; i < numWorkers; i++ {
				go func() { f(); wg.Done() }()
			}

			err = p.Wait()
			if !isContextError(err) {
				t.Fatal(err)
			}

			duration := time.Since(started)
			if duration > passDuration {
				t.Errorf("took %s, expected < %s", duration, passDuration)
			}

			wg.Wait()

			total := counter.Load()
			if total != expected {
				diff := total - expected
				t.Logf("expected %d but got %d (%d)", expected, total, diff)

				if total < expected {
					t.Fatal("did not reach expected target")
				}

				overflowPercentage := (float64(diff) / float64(expected)) * 100
				t.Logf("percentage overflow:%f%%", overflowPercentage)
				if overflowPercentage > maxOverflowPercentage {
					t.Fatalf("overflow %f is greater than allowed %f", overflowPercentage, maxOverflowPercentage)
				}

			}
		})
	}

}

func TestCancel(t *testing.T) {
	p, err := NewPacer(Config{Duration: 10 * time.Second, EPS: 10})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()

	err = p.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Wait()
	if err == nil {
		t.Fatal("expected an error")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}

}

func TestRampup(t *testing.T) {
	pacerDuration := time.Second * 6

	p, err := NewPacer(Config{
		Duration: pacerDuration,
		RampUp:   pacerDuration / 2,
		EPS:      1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = p.Start(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	err = p.Wait()
	if !isContextError(err) {
		t.Fatal(err)
	}
}
