package cache

import (
	"gowebproxy/parser"
)

type Cache struct {
	table map[string]parser.HttpResponse
}

func (c *Cache) Get(method string, resource string) (parser.HttpResponse, bool) {
	key := method + " " + resource

	response, ok := c.table[key]

	return response, ok
}

func (c *Cache) Set(method string, resource string, response parser.HttpResponse) {
	key := method + " " + resource
	c.table[key] = response
}
