package cache

import (
	"errors"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

var loglevel = 0
var cacheListMtx sync.RWMutex

// type cachename string
type cache struct {
	Data map[string]cacheitem
	Lock sync.RWMutex
}

// list of caches by name
var caches = map[string]*cache{}

type cacheitem struct {
	item    interface{}
	expires int64
}

// SetLogLevel sets whether to display cache logging messages: 0=off; 1=hits and misses; 2=trace
func SetLogLevel(level int) {
	loglevel = level
}

// init creates a cache
func initCache(cachename string) *cache {
	logg("Init cache ", cachename)
	c := cache{
		Data: map[string]cacheitem{},
		Lock: sync.RWMutex{},
	}
	cacheListMtx.Lock()
	defer cacheListMtx.Unlock()
	caches[cachename] = &c
	return &c
}

// Put an item in the cache
func Put(cachename string, key string, obj interface{}, ttlSeconds int64) error {
	logg(fmt.Sprintf("Put %v into cache %v", key, cachename))
	cacheListMtx.RLock()
	cc, ok := caches[cachename]
	cacheListMtx.RUnlock()
	if !ok {
		// log.Fatalf("Trying to us cache before Init: %v", cachename)
		cc = initCache(cachename)
	}
	logg(fmt.Sprintf("Waiting to writelock cache %v", cachename))

	cc.Lock.Lock()
	defer cc.Lock.Unlock()
	cc.Data[key] = cacheitem{obj, time.Now().Unix() + ttlSeconds}
	return nil
}

// Get an item from the cche
func Get(cachename string, key string) (interface{}, error) {
	logg(fmt.Sprintf("GET %v %v", cachename, key))

	cacheListMtx.RLock()
	c, ok := caches[cachename]
	cacheListMtx.RUnlock()

	if !ok {
		logg(fmt.Sprintf("No cache called %v", cachename))
		if loglevel >= 1 {
			log.Errorf("----->>>>>>> CACHE MISS: %v %v", cachename, key)
		}
		return nil, errors.New("No cache by that name")
	}
	cc := *&c
	logg(fmt.Sprintf("Waiting to readlock cache %v", cachename))

	// using writelock because of potential expiration
	cc.Lock.RLock()
	// defer cc.Lock.Unlock()

	if _, ok := cc.Data[key]; !ok {
		cc.Lock.RUnlock()
		logg(fmt.Sprintf("No cache entry with key %v in cache %v", key, cachename))
		if loglevel >= 1 {
			log.Errorf("----->>>>>>> CACHE ITEM NOT FOUND: %v %v", cachename, key)
		}
		return nil, errors.New("No cache entry by that key")
	}

	if cc.Data[key].expires < time.Now().Unix() {
		logg(fmt.Sprintf("Cache entry with key %v in cache %v has expired", key, cachename))

		cc.Lock.RUnlock()

		var expiredData interface{}
		cc.Lock.Lock()
		expiredData = cc.Data[key].item
		delete(cc.Data, key)
		cc.Lock.Unlock()
		logg(fmt.Sprintf("Cleared cache entry with key %v in cache %v due to expiration", key, cachename))

		if loglevel >= 1 {
			log.Errorf("----->>>>>>> CACHE ITEM EXPIRED: %v %v [now:%v -> exp:%v]", cachename, key, time.Now().Unix(), cc.Data[key].expires)
		}
		return expiredData, errors.New("cache entry has expired")
	}
	logg(fmt.Sprintf("got %s from cache %s", key, cachename))
	if loglevel >= 1 {
		log.Warnf("----->>>>>>> CACHE HIT: %v %v [ITEM EXPIRES IN %vs]", cachename, key, cc.Data[key].expires-time.Now().Unix())
	}

	item := cc.Data[key].item
	cc.Lock.RUnlock()
	return item, nil
}

func logg(args ...interface{}) {
	if loglevel >= 2 {
		log.Info(args...)
	}
}

// Purge clears entire cache
func Purge(cachename string) {
	logg(fmt.Sprintf("PURGE %v", cachename))

	cacheListMtx.Lock()
	defer cacheListMtx.Unlock()

	if _, ok := caches[cachename]; ok {
		delete(caches, cachename)
	}

}

// Purge clears entire cache
func PurgeAll() {
	logg(fmt.Sprintf("PURGE ALL"))

	cacheListMtx.Lock()
	defer cacheListMtx.Unlock()

	for c := range caches {
		delete(caches, c)
	}

}

// InvalidateItem removes item from cache
func InvalidateItem(cachename string, key string) error {
	logg(fmt.Sprintf("invalidate %v %v", cachename, key))

	cacheListMtx.RLock()
	c, ok := caches[cachename]
	cacheListMtx.RUnlock()

	if !ok {
		logg(fmt.Sprintf("No cache called %v", cachename))
		return errors.New("No cache by that name")
	}
	cc := *&c
	logg(fmt.Sprintf("Waiting to readlock cache %v", cachename))

	cc.Lock.Lock()
	defer cc.Lock.Unlock()
	if _, ok := cc.Data[key]; !ok {
		logg(fmt.Sprintf("No cache entry with key %v in cache %v", key, cachename))

		return errors.New("No cache entry by that key")
	}
	delete(cc.Data, key)
	logg(fmt.Sprintf("Cache entry with key %v in cache %v has been deleted", key, cachename))

	return nil
}
