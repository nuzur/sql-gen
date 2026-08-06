package tosql

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// update rewrites the golden files under testdata/ instead of asserting against
// them: `go test ./tosql/ -run 'TestGenMysql|TestGenPG' -update -count=1`.
//
// The generator's output is a whole directory of SQL files, so a change to the
// resolver or a template touches thousands of golden bytes. Regenerating by hand
// (or, worse, by pasting the diff back in) makes it impossible to tell an intended
// change from an accident. With a flag the workflow is: regenerate, then read
// `git diff tosql/testdata` and attribute every hunk. Anything unattributable is a
// bug, not a golden that needs updating.
var update = flag.Bool("update", false, "rewrite golden files under testdata/")

// assertGolden compares got against the file at path, or rewrites it under
// -update.
func assertGolden(t *testing.T, path string, got string) {
	t.Helper()

	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if !assert.NoErrorf(t, err, "reading golden %s", path) {
		return
	}
	assert.Equalf(t, string(want), got, "golden %s is out of date (re-run with -update to regenerate)", path)
}
