package diffview

import "testing"

func TestLineCache_PutAndGet(t *testing.T) {
	lc := newLineCache(8)
	key := lineKey(LineAdd, ".go", 120, "func main() {")
	lc.Put(key, "rendered-foo")

	got, ok := lc.Get(key)
	if !ok {
		t.Fatal("expected hit, got miss")
	}
	if got != "rendered-foo" {
		t.Errorf("got %q, want rendered-foo", got)
	}
}

func TestLineKey_DistinctOnDifferentInputs(t *testing.T) {
	a := lineKey(LineAdd, ".go", 120, "x := 1")
	b := lineKey(LineDel, ".go", 120, "x := 1")
	c := lineKey(LineAdd, ".ts", 120, "x := 1")
	d := lineKey(LineAdd, ".go", 100, "x := 1")
	e := lineKey(LineAdd, ".go", 120, "x := 2")

	keys := []uint64{a, b, c, d, e}
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] == keys[j] {
				t.Errorf("keys %d and %d collide: %d", i, j, keys[i])
			}
		}
	}
}

func TestLineKey_StableOnSameInputs(t *testing.T) {
	a := lineKey(LineAdd, ".go", 120, "x := 1")
	b := lineKey(LineAdd, ".go", 120, "x := 1")
	if a != b {
		t.Errorf("identical inputs produced different keys: %d vs %d", a, b)
	}
}
