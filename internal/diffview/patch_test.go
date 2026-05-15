package diffview

import "testing"

const sampleDiff = `diff --git a/src/floorplan/wall.tsx b/src/floorplan/wall.tsx
index abc..def 100644
--- a/src/floorplan/wall.tsx
+++ b/src/floorplan/wall.tsx
@@ -10,7 +10,8 @@ export function drawWall(...) {
   const ctx = canvas.getContext('2d')
-  const start = points[0]
+  const start = snapTo(points[0])
+  if (!start) return null
   ctx.beginPath()
diff --git a/src/floorplan/types.ts b/src/floorplan/types.ts
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/src/floorplan/types.ts
@@ -0,0 +1,3 @@
+export type Point = { x: number; y: number }
+export type Wall = { start: Point; end: Point }
+export type Snap = (p: Point) => Point | null
`

func TestParse_TwoFiles(t *testing.T) {
	files := Parse(sampleDiff)
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if files[0].Path != "src/floorplan/wall.tsx" {
		t.Errorf("file 0 path = %q", files[0].Path)
	}
	if files[0].Adds != 2 || files[0].Dels != 1 {
		t.Errorf("file 0 +/-: got +%d -%d, want +2 -1", files[0].Adds, files[0].Dels)
	}
	if files[1].Path != "src/floorplan/types.ts" {
		t.Errorf("file 1 path = %q", files[1].Path)
	}
	if files[1].Adds != 3 || files[1].Dels != 0 {
		t.Errorf("file 1 +/-: got +%d -%d, want +3 -0", files[1].Adds, files[1].Dels)
	}
}

func TestParse_LineKinds(t *testing.T) {
	files := Parse(sampleDiff)
	if len(files) == 0 {
		t.Fatal("no files parsed")
	}
	kinds := map[LineKind]int{}
	for _, l := range files[0].Lines {
		kinds[l.Kind]++
	}
	if kinds[LineHunk] != 1 {
		t.Errorf("expected 1 hunk header, got %d", kinds[LineHunk])
	}
	if kinds[LineAdd] != 2 {
		t.Errorf("expected 2 add lines, got %d", kinds[LineAdd])
	}
	if kinds[LineDel] != 1 {
		t.Errorf("expected 1 del line, got %d", kinds[LineDel])
	}
}

func TestParse_EmptyInput(t *testing.T) {
	files := Parse("")
	if files != nil {
		t.Errorf("empty diff should return nil, got %v", files)
	}
}
