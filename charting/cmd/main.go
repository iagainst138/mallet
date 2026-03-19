package main

import (
	"fmt"
	"math/rand"
	"time"

	"charting"
)

func main() {
	n := 50
	numSerie := 8

	x := make([]time.Time, n)

	serie := make([]charting.Series, numSerie)

	for i := 0; i < numSerie; i++ {
		serie[i] = charting.Series{
			Name:   fmt.Sprintf("series_%d", i),
			Values: make([]int64, n),
		}
	}

	for i := 0; i < n; i++ {
		x[i] = time.Now()

		for _, s := range serie {
			s.Values[i] = int64(rand.Intn(100))
		}
	}

	o := charting.Options{
		Path: "/tmp/chart.html",
		X:    x,

		PageTitle: "Demo",
		Title:     "Title",
		Subtitle:  "---- subtitle ----",

		Serie: serie,
	}

	if err := charting.Chart(o); err != nil {
		panic(err)
	}
}
