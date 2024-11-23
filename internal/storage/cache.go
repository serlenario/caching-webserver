package storage

import (
	"time"

	"github.com/patrickmn/go-cache"
)

var c *cache.Cache

func InitCache() {
	c = cache.New(5*time.Minute, 10*time.Minute)
}

func GetCache() *cache.Cache {
	return c
}
