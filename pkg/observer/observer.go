package observer

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

func random(length int) (string, error) {
	randomData := make([]byte, length)
	_, err := rand.Read(randomData)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(randomData), nil
}

type observer[T any] struct {
	sync.RWMutex
	clients map[string]func(T)
}

func (o *observer[T]) Register(f func(T)) (id string) {
	o.Lock()
	defer o.Unlock()

	id, _ = random(10)

	o.clients[id] = f

	return id
}

func (o *observer[T]) Deregister(id string) {
	o.Lock()
	defer o.Unlock()

	delete(o.clients, id)
}

func (o *observer[T]) Notify(message T) {
	o.RLock()
	defer o.RUnlock()

	for i := range o.clients {
		go o.clients[i](message)
	}
}

func New[T any]() observer[T] {

	return observer[T]{
		clients: make(map[string]func(T)),
	}
}
