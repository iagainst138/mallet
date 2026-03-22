package helpers

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"time"
)

const requestBufferSize = 1000

var additionalEnvVars = []string{
	"PYTHONUNBUFFERED=1",
}

type RequestX struct {
	Method string `json:"method"`
	Query  string `json:"query"`
	Body   string `json:"body"`
}

type Loader struct {
	Command string
	Args    []string

	ready bool

	c chan RequestX
}

func NewLoader(ctx context.Context, command string) (*Loader, error) {
	l := &Loader{
		Command: command,
		c:       make(chan RequestX, requestBufferSize),
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

func (l *Loader) Get(ctx context.Context) (RequestX, error) {
	t := time.NewTimer(time.Millisecond * 10)

	select {
	case r := <-l.c:
		return r, nil
	case <-t.C:
		panic("timed out gettig data")
	}
}

func (l *Loader) Run(ctx context.Context) error {
	env := os.Environ()

	/*cmd := exec.CommandContext(
	ctx,
	*/
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

	n := 0
	for {
		b, err := r.ReadBytes('\n')
		if err != nil {
			//if errors.Is(err, io.EOF) {
			//	return nil
			//}
			//log.Printf("error reading data: %s", err)
			//continue
			return err
		}

		requestBytes := b[:len(b)-1]
		//fmt.Printf("\nGO: %s\n", requestBytes)

		var r RequestX
		if err := json.Unmarshal(requestBytes, &r); err != nil {
			log.Fatal(err)
		}

		l.c <- r
		n++
		if n == requestBufferSize {
			l.ready = true
		}
	}
}
