package mallet

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

const requestBufferSize = 1000

var (
	additionalEnvVars = []string{
		"PYTHONUNBUFFERED=1",
	}

	EmptyRequestBuffer = errors.New("empty request buffer")
)

type Ring struct {
	sync.Mutex

	size       int
	requests   []Request
	readIndex  int
	writeIndex int
}

func NewRing(size int) *Ring {
	return &Ring{
		size:     size,
		requests: make([]Request, 0, size),
	}
}

func (r *Ring) Add(req Request) {
	r.Lock()
	defer r.Unlock()

	if len(r.requests) < r.size {
		r.requests = append(r.requests, req)
	} else {
		r.requests[r.writeIndex] = req
	}

	r.writeIndex++
	if r.writeIndex == r.size {
		r.writeIndex = 0
	}
}

func (r *Ring) Get() (Request, bool) {
	r.Lock()
	defer r.Unlock()

	if len(r.requests) == 0 {
		return Request{}, false
	}

	r.readIndex++
	if r.readIndex == len(r.requests) {
		r.readIndex = 0
	}
	return r.requests[r.readIndex], true
}

func (r *Ring) hasEntries() bool {
	return r != nil && len(r.requests) > 0
}

type Request struct {
	Method string `json:"method"`
	Query  string `json:"query"`
	Body   string `json:"body"`

	Unique bool `json:"unique"`
}

type Loader struct {
	Command string
	Args    []string

	requestBuffer *Ring

	ready bool

	c chan Request
}

func NewLoader(ctx context.Context, command string) (*Loader, error) {
	l := &Loader{
		Command:       command,
		c:             make(chan Request, requestBufferSize),
		requestBuffer: NewRing(200),
	}

	go func() {
		err := l.Run(ctx)
		if err != nil {
			panic(err)
		}
	}()

	if err := l.Ready(); err != nil {
		return nil, err
	}

	return l, nil
}

func (l *Loader) Ready() error {
	// TODO make better
	for !l.ready {
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func (l *Loader) Get(ctx context.Context) (Request, error) {
	select {
	case r := <-l.c:
		return r, nil
	default:
		if r, ok := l.requestBuffer.Get(); ok {
			return r, nil
		} else {
			return Request{}, EmptyRequestBuffer
		}
	}
}

func (l *Loader) Run(ctx context.Context) error {
	env := os.Environ()

	cmd := exec.Command(
		l.Command,
		l.Args...,
	)
	cmd.Env = append(env, additionalEnvVars...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	r := bufio.NewReader(stdout)

	for {
		b, err := r.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && l.requestBuffer.hasEntries() {
				return nil
			}
			return err
		}

		requestBytes := b[:len(b)-1]

		var r Request
		if err := json.Unmarshal(requestBytes, &r); err != nil {
			log.Fatal(err)
		}

		l.c <- r
		if !r.Unique {
			l.ready = true
			l.requestBuffer.Add(r)
		}
	}
}
