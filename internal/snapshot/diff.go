package snapshot

import (
	"bufio"
	"strings"

	"github.com/alexandrmotologa/sandforge/internal/types"
)

// ParseGitDiff parses unified git diff output into structured DiffEntry items.
func ParseGitDiff(raw string) []types.DiffEntry {
	if strings.TrimSpace(raw) == "" {
		return []types.DiffEntry{}
	}

	var entries []types.DiffEntry
	scanner := bufio.NewScanner(strings.NewReader(raw))

	var currentEntry *types.DiffEntry
	var currentPatch strings.Builder

	flush := func() {
		if currentEntry != nil {
			currentEntry.Patch = currentPatch.String()
			entries = append(entries, *currentEntry)
			currentEntry = nil
			currentPatch.Reset()
		}
	}

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "diff --git ") {
			flush()
			parts := strings.Split(line, " ")
			var path string
			if len(parts) >= 4 {
				path = strings.TrimPrefix(parts[3], "b/")
			}
			currentEntry = &types.DiffEntry{
				Path:   path,
				Status: "modified",
			}
			currentPatch.WriteString(line + "\n")
			continue
		}

		if currentEntry == nil {
			continue
		}

		currentPatch.WriteString(line + "\n")

		if strings.HasPrefix(line, "new file mode ") {
			currentEntry.Status = "added"
		} else if strings.HasPrefix(line, "deleted file mode ") {
			currentEntry.Status = "deleted"
		} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			currentEntry.Additions++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			currentEntry.Deletions++
		}
	}
	flush()

	return entries
}
