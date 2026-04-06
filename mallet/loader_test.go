package mallet

import (
	"fmt"
	"testing"
)

func TestRing(t *testing.T) {
	size := 200
	ring := NewRing(size)

	if _, ok := ring.Get(); ok {
		t.Fatal("did not expect to get anything from ring")
	}

	for i := range size * 2 {
		ring.Add(Request{Query: fmt.Sprintf("%d", i)})

		_, ok := ring.Get()
		if !ok {
			t.Fatal("expected to get Request from ring")
		}
		//for _, x := range ring.requests {
		//	fmt.Printf("%+v\n", x)
		//}
		//fmt.Printf("[loop 1] index:%d | req:%+v\n\n", i, r)
	}

	//fmt.Println("")
	//panic("!!!")

	for i := range size {
		r, ok := ring.Get()
		if !ok {
			t.Fatal("expected to get Request from ring")
		}

		j := fmt.Sprintf("%d", i+size)
		//fmt.Println(i, j, r)

		if r.Query != j {
			t.Fatalf("expected %s but got %s", j, r.Query)
		}
	}

	ringSize := len(ring.requests)
	if ringSize != size {
		t.Fatalf("expected %d but got %d", size, ringSize)
	}
}
