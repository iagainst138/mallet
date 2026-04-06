package mallet

import (
	"fmt"
	"testing"
)

func TestRequestBuffer(t *testing.T) {
	size := 200
	requestBuffer := NewRequestBuffer(size)

	if _, ok := requestBuffer.Get(); ok {
		t.Fatal("did not expect to get anything from requestBuffer")
	}

	for i := range size * 2 {
		requestBuffer.Add(Request{Query: fmt.Sprintf("%d", i)})

		_, ok := requestBuffer.Get()
		if !ok {
			t.Fatal("expected to get Request from requestBuffer")
		}
	}

	for i := range size {
		r, ok := requestBuffer.Get()
		if !ok {
			t.Fatal("expected to get Request from requestBuffer")
		}

		j := fmt.Sprintf("%d", i+size)

		if r.Query != j {
			t.Fatalf("expected %s but got %s", j, r.Query)
		}
	}

	ringSize := len(requestBuffer.requests)
	if ringSize != size {
		t.Fatalf("expected %d but got %d", size, ringSize)
	}
}
