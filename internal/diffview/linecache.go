package diffview

import (
	"strconv"
	"sync"

	"github.com/cespare/xxhash/v2"
	lru "github.com/hashicorp/golang-lru/v2"
)

// lineCache memoizes fully-rendered styled diff lines keyed by content hash.
// Two PRs that touch the same line of the same kind at the same width share
// one cached render — the canonical example is a dependency bump appearing in
// 12 package.json files across a stack.
type lineCache struct {
	mu sync.Mutex
	c  *lru.Cache[uint64, string]
}

func newLineCache(size int) *lineCache {
	c, _ := lru.New[uint64, string](size)
	return &lineCache{c: c}
}

// key folds the line kind, file extension (drives lexer choice), pane width,
// and raw text into a single hash. Anything that would change the rendered
// output goes into the key; nothing else.
func lineKey(kind LineKind, ext string, width int, text string) uint64 {
	h := xxhash.New()
	_, _ = h.WriteString(strconv.Itoa(int(kind)))
	_, _ = h.WriteString("|")
	_, _ = h.WriteString(ext)
	_, _ = h.WriteString("|")
	_, _ = h.WriteString(strconv.Itoa(width))
	_, _ = h.WriteString("|")
	_, _ = h.WriteString(text)
	return h.Sum64()
}

func (lc *lineCache) Get(key uint64) (string, bool) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	return lc.c.Get(key)
}

func (lc *lineCache) Put(key uint64, value string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.c.Add(key, value)
}

