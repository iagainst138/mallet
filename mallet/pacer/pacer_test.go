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

		{eps: 60000, pacerDuration: time.Second * 10, long: true},
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

			p, err := NewPacer(PacerConfig{Duration: tc.pacerDuration, EPS: tc.eps})
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

			started := time.Now()
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(tc.pacerDuration))
			defer cancel()
			err = p.Start(ctx)
			if err != nil {
				t.Fatal(err)
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
				var diff int64

				if total < expected {
					diff = expected - total
				} else {
					diff = total - expected
				}

				overflowPercentage := (float64(diff) / float64(expected)) * 100

				t.Logf("expected %d but got %d (difference: %d - %f%%)", expected, total, diff, overflowPercentage)
				if overflowPercentage > maxOverflowPercentage {
					t.Fatalf("overflow %f is greater than allowed %f", overflowPercentage, maxOverflowPercentage)
				}

			}
		})
	}

}

func TestCancel(t *testing.T) {
	p, err := NewPacer(PacerConfig{Duration: 10 * time.Second, EPS: 10})
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

	p, err := NewPacer(PacerConfig{
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

func TestPercentageComplete(t *testing.T) {
	p := Pacer{
		Duration: time.Millisecond * 200,
		started:  time.Now(),
	}

	if p.PercentageComplete() == 0 {
		t.Fatal("expected non zero result")
	}

	time.Sleep(p.Duration)

	if r := p.PercentageComplete(); r != 100 {
		t.Fatalf("expected 100 got %f", r)
	}
}

// TestDoSpreadsTokens checks a segment's tokens are handed out across the
// segment rather than all at its start.
func TestDoSpreadsTokens(t *testing.T) {
	const n = 10
	const segment = 100 * time.Millisecond

	p, err := NewPacer(PacerConfig{EPS: 1})
	if err != nil {
		t.Fatal(err)
	}

	var times []time.Time
	done := make(chan struct{})
	go func() {
		for range p.ch {
			times = append(times, time.Now())
		}
		close(done)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), segment)
	defer cancel()

	started := time.Now()
	if left := p.do(ctx, n); left != 0 {
		t.Fatalf("expected every token to be handed out, %d left", left)
	}
	close(p.ch)
	<-done

	if len(times) != n {
		t.Fatalf("expected %d tokens, got %d", n, len(times))
	}

	// Due at 0, 10ms ... 90ms: the last is most of a segment after the first.
	if span := times[n-1].Sub(times[0]); span < segment*7/10 {
		t.Fatalf("tokens spanned %s of a %s segment, want them spread across it", span, segment)
	}
	spacing := segment / n
	for i, at := range times {
		if due := started.Add(time.Duration(i) * spacing); at.Before(due) {
			t.Fatalf("token %d handed out %s before it was due", i, due.Sub(at))
		}
	}
}

// TestPacerSpreadsRequests checks the pacer delivers its rate as a steady
// stream: no 10ms window gets much more than its share, where handing out
// whole segments at once put 100 tokens into the first millisecond of each.
//
// One receiver takes every token, so the times are when the pacer handed them
// out. With many, a token reaches a parked worker the moment it is sent but is
// only timed once the scheduler runs that worker, which bunches the times up
// however evenly the tokens went out.
func TestPacerSpreadsRequests(t *testing.T) {
	const eps = 800
	const window = 10 * time.Millisecond
	const share = eps * window / time.Second // 8 per window

	p, err := NewPacer(PacerConfig{Duration: time.Second, EPS: eps})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	var times []time.Time
	done := make(chan struct{})
	go func() {
		for p.Get() == nil {
			times = append(times, time.Now())
		}
		close(done)
	}()
	if err := p.Wait(); !isContextError(err) {
		t.Fatal(err)
	}
	<-done

	if len(times) < eps*9/10 {
		t.Fatalf("expected about %d tokens, got %d", eps, len(times))
	}

	most, j := 0, 0
	for i := range times {
		for times[i].Sub(times[j]) >= window {
			j++
		}
		most = max(most, i-j+1)
	}
	if most > 3*int(share) {
		t.Fatalf("%d tokens in one %s window, want about %d", most, window, share)
	}
	t.Logf("busiest %s window: %d tokens (share %d)", window, most, share)
}
