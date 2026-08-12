package server

import (
	"fmt"
	"sync"
	"testing"
)

func TestHubConcurrentAddRemove(t *testing.T) {
	hub := NewHub()

	const goroutines = 32
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := range goroutines {
		go func(base int) {
			defer wg.Done()
			for i := range iterations {
				id := fmt.Sprintf("client-%d-%d", base, i)
				c := &ServerClient{ID: id}
				hub.addClient(c)
				_ = hub.getClient(id)
				hub.removeClient(id)
			}
		}(g)
	}

	wg.Wait()
}
