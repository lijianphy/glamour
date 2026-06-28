package ansi

import (
	"container/list"
	"crypto/sha256"
	"sync"
)

const (
	codeBlockRenderCacheMaxEntries = 256
	codeBlockRenderCacheMaxBytes   = 4 << 20
)

type codeBlockCacheKey struct {
	hash       [sha256.Size]byte
	length     int
	language   string
	formatter  string
	theme      string
	background string
	width      int
	faint      bool
}

type codeBlockCacheEntry struct {
	key  codeBlockCacheKey
	text string
	size int
}

type codeBlockRenderCache struct {
	mu      sync.Mutex
	ll      *list.List
	entries map[codeBlockCacheKey]*list.Element
	bytes   int
}

func newCodeBlockRenderCache() *codeBlockRenderCache {
	return &codeBlockRenderCache{
		ll:      list.New(),
		entries: make(map[codeBlockCacheKey]*list.Element),
	}
}

func newCodeBlockCacheKey(source, language, formatter, theme, background string, width int, faint bool) codeBlockCacheKey {
	return codeBlockCacheKey{
		hash:       sha256.Sum256([]byte(source)),
		length:     len(source),
		language:   language,
		formatter:  formatter,
		theme:      theme,
		background: background,
		width:      width,
		faint:      faint,
	}
}

func (c *codeBlockRenderCache) Get(key codeBlockCacheKey) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[key]
	if !ok {
		return "", false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*codeBlockCacheEntry).text, true
}

func (c *codeBlockRenderCache) Add(key codeBlockCacheKey, text string) {
	if c == nil {
		return
	}
	size := len(text) + key.length + len(key.language) + len(key.formatter) + len(key.theme) + len(key.background)
	if size > codeBlockRenderCacheMaxBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[key]; ok {
		entry := el.Value.(*codeBlockCacheEntry)
		c.bytes += size - entry.size
		entry.text = text
		entry.size = size
		c.ll.MoveToFront(el)
		c.evict()
		return
	}
	entry := &codeBlockCacheEntry{key: key, text: text, size: size}
	c.entries[key] = c.ll.PushFront(entry)
	c.bytes += size
	c.evict()
}

func (c *codeBlockRenderCache) evict() {
	for len(c.entries) > codeBlockRenderCacheMaxEntries || c.bytes > codeBlockRenderCacheMaxBytes {
		el := c.ll.Back()
		if el == nil {
			return
		}
		entry := el.Value.(*codeBlockCacheEntry)
		delete(c.entries, entry.key)
		c.bytes -= entry.size
		c.ll.Remove(el)
	}
}
