#!/usr/bin/env bash
# setup-partial-demo.sh — create a throwaway git repo with a rich set of
# working-tree changes for exercising nidhi's PARTIAL (partial stash) screen.
#
# Produces:
#   - auth.go : a modified file with TWO separate hunks (import block + body)
#   - db.go   : a modified file with ONE hunk
#   - notes.md: a PRE-STAGED change (git add) to verify staged state survives
#
# Usage: setup-partial-demo.sh [DIR]   (default: /tmp/nidhi-partial-demo)
set -euo pipefail

DIR="${1:-/tmp/nidhi-partial-demo}"
rm -rf "$DIR"
mkdir -p "$DIR"
cd "$DIR"

git init -q
git config user.email demo@nidhi.test
git config user.name "nidhi demo"

cat > auth.go <<'EOF'
package auth

func Refresh(t string) string {
	return t
}

func Validate(t string) bool {
	return t != ""
}
EOF

cat > db.go <<'EOF'
package db

func Connect() error {
	return nil
}
EOF

cat > notes.md <<'EOF'
# Notes
initial
EOF

git add .
git commit -qm "base"

# auth.go: two distinct hunks (top import block + bottom Validate body).
cat > auth.go <<'EOF'
package auth

import "fmt"

func Refresh(t string) string {
	fmt.Println("refreshing")
	return t + "-new"
}

func Validate(t string) bool {
	return len(t) > 3
}
EOF

# db.go: a single hunk.
cat > db.go <<'EOF'
package db

func Connect() error {
	// pool the connection
	return nil
}
EOF

# notes.md: a change that we STAGE, to prove pre-existing staged state is
# preserved across a partial stash of the OTHER files.
printf '# Notes\ninitial\nstaged line\n' > notes.md
git add notes.md

echo "demo repo ready at: $DIR"
echo "--- git status --short ---"
git status --short
echo "--- diff hunk count (unstaged vs HEAD) ---"
git --no-pager diff | grep -c '^@@' || true
