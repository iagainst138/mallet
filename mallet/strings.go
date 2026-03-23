package mallet

import (
	"math/rand/v2"
	"strings"
)

var letters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandomString(n int) string {
	sb := strings.Builder{}
	sb.Grow(n)
	for range n {
		x := rand.IntN(len(letters) - 1)
		sb.Write(letters[x : x+1])
	}

	return sb.String()
}
