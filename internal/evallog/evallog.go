// Package evallog persists per-hook evaluation results as JSONL for later
// aggregation by `ccchain stats` (Plan 0029 / Issue #17).
//
// Design constraints:
//
//   - **Fail-open.** Log write errors are returned to the caller so that the
//     hook can log a stderr warning; they must NEVER change the allow/deny
//     decision. `Log` never panics on a bad path.
//   - **Append-only.** JSONL, one entry per line, O_APPEND. POSIX guarantees
//     atomic writes for `write(2)` calls under PIPE_BUF (>= 512 bytes on all
//     modern platforms) so short entries do not need an external lock even
//     under concurrent hook invocations. Long lines can still interleave in
//     principle; we bound this by truncating `Command` before serialization.
//   - **Owner-only perms.** Files are created 0600 and the parent directory
//     0700, matching the approval store's threat model. O_NOFOLLOW guards
//     against symlink attacks on the file itself.
//   - **Secret containment.** `Command` is truncated to `CommandLengthLimit`
//     bytes (default 200) at rune boundaries so a UTF-8 truncation cannot
//     leave a broken sequence in the log.
package evallog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
	"unicode/utf8"
)

// CommandLengthLimit bounds how many bytes of the raw command are recorded.
// Chosen to keep entries below the POSIX PIPE_BUF (512 bytes for a portable
// atomic write) once the surrounding JSON overhead is added, and to keep
// accidental secret leakage bounded even before conf-side truncation.
const CommandLengthLimit = 200

// Entry is the JSONL row appended by Log. Field names are stable — external
// tools (stats aggregators, dashboards) rely on the schema. Do NOT rename
// without a migration path.
type Entry struct {
	Timestamp      int64  `json:"timestamp"`
	ToolName       string `json:"tool_name"`
	Command        string `json:"command,omitempty"`
	Action         string `json:"action"`
	MatchedRule    string `json:"matched_rule,omitempty"`
	Message        string `json:"message,omitempty"`
	PermissionMode string `json:"permission_mode,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	CWD            string `json:"cwd,omitempty"`
}

// Log appends one Entry to path. The parent directory is created 0700 if
// missing. Errors are wrapped with the file base name so the caller can log
// a concise stderr warning without leaking the resolved path.
//
// Timestamp defaults to time.Now().Unix() if zero.
func Log(path string, entry Entry) error {
	if path == "" {
		return errors.New("evallog: empty path")
	}
	if entry.Timestamp == 0 {
		entry.Timestamp = time.Now().Unix()
	}
	entry.Command = truncateCommand(entry.Command, CommandLengthLimit)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("evallog: mkdir %s: %w", filepath.Base(dir), err)
	}
	// Tighten in case MkdirAll used a broader umask; ignore error because the
	// dir may be owned by another user (a global log location) — the mode
	// there is the operator's choice.
	_ = os.Chmod(dir, 0o700)

	f, err := os.OpenFile(path,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return fmt.Errorf("evallog: open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()

	// json.Encoder appends a newline; that plus O_APPEND is the JSONL append
	// contract.
	buf, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("evallog: marshal: %w", err)
	}
	buf = append(buf, '\n')
	if _, err := f.Write(buf); err != nil {
		return fmt.Errorf("evallog: write: %w", err)
	}
	return nil
}

// truncateCommand caps s at max bytes on a rune boundary. UTF-8 truncation
// mid-sequence would leave 0xff / replacement characters in the log which are
// harder to grep and can break naive column alignment.
func truncateCommand(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	// Walk back from max to the first valid rune boundary.
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// Aggregate is one row of the stats table: how many events shared Key and
// when the last one landed.
type Aggregate struct {
	Key      string `json:"key"`
	Count    int    `json:"count"`
	LastSeen int64  `json:"last_seen"`
}

// GroupBy selects the aggregation dimension.
type GroupBy string

const (
	GroupByAction  GroupBy = "action"
	GroupByRule    GroupBy = "rule"
	GroupByCommand GroupBy = "command"
)

// Stats reads the JSONL log at path and returns aggregates over entries
// whose Timestamp is within `since` from now. Pass 0 for "all entries".
// Results are sorted by Count desc, then LastSeen desc, then Key asc so
// the ordering is deterministic across runs.
//
// If path does not exist, Stats returns (nil, nil): an empty log is a
// legitimate state (nothing has been evaluated yet).
func Stats(path string, since time.Duration, groupBy GroupBy) ([]Aggregate, error) {
	entries, err := readEntries(path)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	cutoff := int64(0)
	if since > 0 {
		cutoff = time.Now().Unix() - int64(since.Seconds())
	}

	agg := map[string]*Aggregate{}
	for _, e := range entries {
		if e.Timestamp < cutoff {
			continue
		}
		key := groupKey(e, groupBy)
		if key == "" {
			// Rows without a groupable value (e.g. group by rule on an
			// unmatched command) go into a distinct bucket so the operator
			// can see they exist rather than silently dropping.
			key = "(none)"
		}
		a := agg[key]
		if a == nil {
			a = &Aggregate{Key: key}
			agg[key] = a
		}
		a.Count++
		if e.Timestamp > a.LastSeen {
			a.LastSeen = e.Timestamp
		}
	}

	out := make([]Aggregate, 0, len(agg))
	for _, a := range agg {
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if out[i].LastSeen != out[j].LastSeen {
			return out[i].LastSeen > out[j].LastSeen
		}
		return out[i].Key < out[j].Key
	})
	return out, nil
}

func groupKey(e Entry, by GroupBy) string {
	switch by {
	case GroupByRule:
		return e.MatchedRule
	case GroupByCommand:
		return e.Command
	default:
		return e.Action
	}
}

// readEntries reads a JSONL log; missing file returns (nil, nil).
func readEntries(path string) ([]Entry, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("evallog: open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()

	var out []Entry
	scanner := bufio.NewScanner(f)
	// Commands can grow up to CommandLengthLimit but Message / paths can be
	// larger; 1MB matches the approval store's buffer size and is way over
	// realistic entry length.
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	line := 0
	for scanner.Scan() {
		line++
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(raw, &e); err != nil {
			// Skip malformed lines but do not fail the aggregation — a partial
			// write from a crashed process should not blind the operator.
			// The count is quietly conservative; a future --strict flag can
			// change this.
			continue
		}
		out = append(out, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("evallog: scan %s: %w", filepath.Base(path), err)
	}
	return out, nil
}
