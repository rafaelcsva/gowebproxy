package cache

import (
	"gowebproxy/parser"
	"sync"
)

type Cache struct {
	table map[string]parser.HttpResponse
	mux   sync.Mutex
}

func NewCache() Cache {
	return Cache{table: make(map[string]parser.HttpResponse)}
}

func (c *Cache) Get(method string, resource string) (parser.HttpResponse, bool) {
	key := method + " " + resource

	c.mux.Lock()
	response, ok := c.table[key]
	c.mux.Unlock()

	return response, ok
}

func (c *Cache) Set(method string, resource string, response parser.HttpResponse) {
	key := method + " " + resource
	c.mux.Lock()
	c.table[key] = response
	c.mux.Unlock()
}
