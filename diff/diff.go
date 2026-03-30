// Package diff parses git diffs and maps changes to graph nodes.
package diff

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/JSLEEKR/codegraph-go/graph"
)

// LineRange represents a range of changed lines in a file.
type LineRange struct {
	Start int
	End   int
}

// ChangedRanges maps file paths to their changed line ranges.
type ChangedRanges map[string][]LineRange

var (
	hunkRe  = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
	fileRe  = regexp.MustCompile(`^\+\+\+ b/(.+)`)
	safeRef = regexp.MustCompile(`^[A-Za-z0-9_.~^/@{}\-]+$`)
)

// ParseGitDiff runs git diff and returns changed line ranges per file.
func ParseGitDiff(repoRoot, base string) (ChangedRanges, error) {
	if base == "" {
		base = "HEAD~1"
	}

	if !safeRef.MatchString(base) {
		return nil, fmt.Errorf("invalid git ref: %q", base)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "diff", "--unified=0", base)
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	return ParseUnifiedDiff(string(output)), nil
}

// ParseGitDiffStaged runs git diff --staged and returns changed line ranges.
func ParseGitDiffStaged(repoRoot string) (ChangedRanges, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "diff", "--unified=0", "--staged")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --staged: %w", err)
	}

	return ParseUnifiedDiff(string(output)), nil
}

// ParseUnifiedDiff parses unified diff text into changed line ranges.
func ParseUnifiedDiff(diffText string) ChangedRanges {
	result := make(ChangedRanges)
	var currentFile string

	for _, line := range strings.Split(diffText, "\n") {
		if m := fileRe.FindStringSubmatch(line); m != nil {
			currentFile = m[1]
			continue
		}

		if currentFile == "" {
			continue
		}

		if m := hunkRe.FindStringSubmatch(line); m != nil {
			start, _ := strconv.Atoi(m[1])
			count := 1
			if m[2] != "" {
				count, _ = strconv.Atoi(m[2])
			}
			if count == 0 {
				count = 1
			}
			result[currentFile] = append(result[currentFile], LineRange{
				Start: start,
				End:   start + count - 1,
			})
		}
	}

	return result
}

// MapChangesToNodes finds graph nodes that overlap with changed line ranges.
func MapChangesToNodes(g *graph.Graph, ranges ChangedRanges) []*graph.Node {
	seen := make(map[string]bool)
	var result []*graph.Node

	for filePath, lineRanges := range ranges {
		nodes := g.GetNodesByFile(filePath)
		for _, node := range nodes {
			if node.Kind == graph.KindFile {
				continue
			}
			for _, lr := range lineRanges {
				if node.LineStart <= lr.End && node.LineEnd >= lr.Start {
					if !seen[node.QualifiedName] {
						seen[node.QualifiedName] = true
						result = append(result, node)
					}
					break
				}
			}
		}
	}

	return result
}

// GetChangedFiles returns just the file paths from changed ranges.
func GetChangedFiles(ranges ChangedRanges) []string {
	files := make([]string, 0, len(ranges))
	for f := range ranges {
		files = append(files, f)
	}
	return files
}
