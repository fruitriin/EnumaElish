package approve

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Scope classifies how strictly an approval must match its consumer.
type Scope string

const (
	// ScopeSession requires session_id AND cwd to match.
	ScopeSession Scope = "session"
	// ScopeGlobal matches on hash alone; use with caution.
	ScopeGlobal Scope = "global"
)

// DefaultTTL is the approval lifetime when the caller does not override it.
const DefaultTTL = 15 * time.Minute

// storeEnvVar is the test hook to point the store at a temporary directory.
const storeEnvVar = "CCCHAIN_APPROVE_STORE"

// PendingEntry records a deny that occurred because an ask degraded under a
// non-interactive mode. Written by the hook; read by `ccchain approve`.
type PendingEntry struct {
	Hash      string `json:"hash"`
	Command   string `json:"command"` // canonical form (safe: no runtime expansion)
	CWD       string `json:"cwd"`
	SessionID string `json:"session_id,omitempty"`
	Timestamp int64  `json:"timestamp"` // unix seconds
}

// ApprovedEntry records a human-issued approval. Consumption is one-shot
// (Consumed=true after the hook lets the command through once).
type ApprovedEntry struct {
	Hash       string `json:"hash"`
	Command    string `json:"command"`
	ApprovedAt int64  `json:"approved_at"` // unix seconds
	TTLSeconds int64  `json:"ttl_seconds"`
	Scope      Scope  `json:"scope"`
	CWD        string `json:"cwd,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	Consumed   bool   `json:"consumed"`
}

// AuditEvent records approval/consumption/revocation events for post-hoc
// review. Append-only.
type AuditEvent struct {
	Timestamp int64  `json:"timestamp"`
	Event     string `json:"event"` // "approve" | "consume" | "revoke"
	Hash      string `json:"hash,omitempty"`
	Command   string `json:"command,omitempty"`
	Scope     Scope  `json:"scope,omitempty"`
	CWD       string `json:"cwd,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Note      string `json:"note,omitempty"`
}

// Store persists pending/approved entries and audit events under dir.
// Concurrent access is serialized by an O_EXCL lock file (short retry, no
// unbounded wait). Files are created with 0600 perms and the parent dir 0700.
type Store struct {
	dir string
	now func() time.Time // injectable for tests
}

// DefaultDir resolves the store directory using CLAUDE_CONFIG_DIR (if set to
// an absolute path) with a home-directory fallback. Test callers may override
// via the CCCHAIN_APPROVE_STORE env var — used only in tests, so it wins.
func DefaultDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv(storeEnvVar)); v != "" {
		return v, nil
	}
	if cd := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); cd != "" && filepath.IsAbs(cd) {
		return filepath.Join(cd, "ccchain"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".claude", "ccchain"), nil
}

// NewStore returns a Store rooted at dir. NewStore does not create the
// directory; that happens lazily on the first write.
func NewStore(dir string) *Store {
	return &Store{dir: dir, now: time.Now}
}

// Dir returns the store's root directory.
func (s *Store) Dir() string { return s.dir }

// SetNow overrides the clock. Test-only.
func (s *Store) SetNow(fn func() time.Time) { s.now = fn }

func (s *Store) pendingPath() string  { return filepath.Join(s.dir, "pending.jsonl") }
func (s *Store) approvedPath() string { return filepath.Join(s.dir, "approved.jsonl") }
func (s *Store) auditPath() string    { return filepath.Join(s.dir, "audit.jsonl") }
func (s *Store) lockPath() string     { return filepath.Join(s.dir, "store.lock") }

// ensureDir creates the store dir with 0700 perms and rejects a dir that is a
// symlink (basic defense against a symlink attack on the store root).
func (s *Store) ensureDir() error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("mkdir store: %w", err)
	}
	info, err := os.Lstat(s.dir)
	if err != nil {
		return fmt.Errorf("lstat store: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("store dir is a symlink: %s (refusing to follow)", s.dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("store path is not a directory: %s", s.dir)
	}
	// Tighten perms in case MkdirAll used a broader umask.
	_ = os.Chmod(s.dir, 0o700)
	return nil
}

// withLock acquires the store lock, runs fn, and releases. Retries a few
// times with short backoff on contention rather than waiting forever.
func (s *Store) withLock(fn func() error) error {
	if err := s.ensureDir(); err != nil {
		return err
	}
	const maxAttempts = 20
	const backoff = 50 * time.Millisecond
	var lockFile *os.File
	var lockErr error
	for i := 0; i < maxAttempts; i++ {
		f, err := os.OpenFile(s.lockPath(),
			os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0o600)
		if err == nil {
			lockFile = f
			lockErr = nil
			break
		}
		lockErr = err
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("acquire lock: %w", err)
		}
		time.Sleep(backoff)
	}
	if lockFile == nil {
		return fmt.Errorf("acquire lock: %w", lockErr)
	}
	defer func() {
		_ = lockFile.Close()
		_ = os.Remove(s.lockPath())
	}()
	return fn()
}

// appendJSONL appends a single JSON-encoded entry as a line to path. The file
// is created 0600 if missing. Uses O_NOFOLLOW so a symlink attack targeting
// the file itself fails to open (defense-in-depth on top of ensureDir).
func appendJSONL(path string, entry any) error {
	f, err := os.OpenFile(path,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	if err := enc.Encode(entry); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	return nil
}

// readJSONL reads a JSONL file into a slice via decode. Returns (nil, nil) if
// the file does not exist (empty store).
func readJSONL[T any](path string) ([]T, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	var out []T
	scanner := bufio.NewScanner(f)
	// JSONL lines may exceed the default 64k for long commands.
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry T
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
		}
		out = append(out, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", filepath.Base(path), err)
	}
	return out, nil
}

// writeJSONLAtomic overwrites path with all entries, via temp file + rename.
// Under lock this is the safe way to prune/mark-consumed a JSONL log.
func writeJSONLAtomic[T any](path string, entries []T) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName) // no-op after successful rename
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	enc := json.NewEncoder(tmp)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("encode: %w", err)
		}
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

// RecordPending appends a pending deny event to the store.
func (s *Store) RecordPending(entry PendingEntry) error {
	if entry.Hash == "" {
		return errors.New("RecordPending: empty hash")
	}
	if entry.Timestamp == 0 {
		entry.Timestamp = s.now().Unix()
	}
	return s.withLock(func() error {
		return appendJSONL(s.pendingPath(), entry)
	})
}

// ListPending returns all recorded pending entries (oldest first).
func (s *Store) ListPending() ([]PendingEntry, error) {
	var out []PendingEntry
	err := s.withLock(func() error {
		entries, err := readJSONL[PendingEntry](s.pendingPath())
		if err != nil {
			return err
		}
		out = entries
		return nil
	})
	return out, err
}

// ApproveOptions bundles the caller's approval intent.
type ApproveOptions struct {
	TTL       time.Duration
	Scope     Scope
	CWD       string
	SessionID string
	Note      string // free-form audit note (e.g. "--last")
}

// applyDefaults fills unset options with safe defaults.
func (o *ApproveOptions) applyDefaults() {
	if o.TTL <= 0 {
		o.TTL = DefaultTTL
	}
	if o.Scope == "" {
		o.Scope = ScopeSession
	}
}

// ApproveLast approves the most recent pending entry, matching sentinel rules
// noted in the plan (one-shot, TTL, scope). Returns the approval and the
// pending entry that seeded it.
func (s *Store) ApproveLast(opts ApproveOptions) (*ApprovedEntry, *PendingEntry, error) {
	opts.applyDefaults()
	var approved *ApprovedEntry
	var seed *PendingEntry
	err := s.withLock(func() error {
		pending, err := readJSONL[PendingEntry](s.pendingPath())
		if err != nil {
			return err
		}
		if len(pending) == 0 {
			return errors.New("no pending approvals")
		}
		last := pending[len(pending)-1]
		seed = &last
		entry := s.buildApproved(last, opts)
		if err := s.appendApprovedLocked(entry); err != nil {
			return err
		}
		approved = &entry
		return nil
	})
	return approved, seed, err
}

// ApproveByHashPrefix approves a pending entry whose hash starts with prefix.
// Ambiguous prefixes (multiple distinct hashes match) return an error. If the
// same hash appears in multiple pending entries, the most recent is used.
func (s *Store) ApproveByHashPrefix(prefix string, opts ApproveOptions) (*ApprovedEntry, *PendingEntry, error) {
	opts.applyDefaults()
	prefix = strings.TrimSpace(prefix)
	if len(prefix) < 4 {
		return nil, nil, fmt.Errorf("hash prefix too short (min 4 chars): %q", prefix)
	}
	var approved *ApprovedEntry
	var seed *PendingEntry
	err := s.withLock(func() error {
		pending, err := readJSONL[PendingEntry](s.pendingPath())
		if err != nil {
			return err
		}
		// Collect matching hashes and their most recent entry.
		latest := map[string]PendingEntry{}
		for _, e := range pending {
			if strings.HasPrefix(e.Hash, prefix) {
				latest[e.Hash] = e
			}
		}
		switch len(latest) {
		case 0:
			return fmt.Errorf("no pending entry matches hash prefix %q", prefix)
		case 1:
			// fall through
		default:
			hashes := make([]string, 0, len(latest))
			for h := range latest {
				hashes = append(hashes, h[:min(len(h), 12)])
			}
			return fmt.Errorf("ambiguous hash prefix %q matches %d entries: %s",
				prefix, len(latest), strings.Join(hashes, ", "))
		}
		var matched PendingEntry
		for _, e := range latest {
			matched = e
		}
		seed = &matched
		entry := s.buildApproved(matched, opts)
		if err := s.appendApprovedLocked(entry); err != nil {
			return err
		}
		approved = &entry
		return nil
	})
	return approved, seed, err
}

// buildApproved constructs an ApprovedEntry from a pending seed + options.
// Under session scope, cwd/session_id fall back to the seed's values so an
// operator can approve without knowing them; the plan explicitly allows this.
func (s *Store) buildApproved(seed PendingEntry, opts ApproveOptions) ApprovedEntry {
	cwd := opts.CWD
	sessionID := opts.SessionID
	if opts.Scope == ScopeSession {
		if cwd == "" {
			cwd = seed.CWD
		}
		if sessionID == "" {
			sessionID = seed.SessionID
		}
	} else {
		// Global scope carries no cwd/session_id — the check ignores them.
		cwd = ""
		sessionID = ""
	}
	return ApprovedEntry{
		Hash:       seed.Hash,
		Command:    seed.Command,
		ApprovedAt: s.now().Unix(),
		TTLSeconds: int64(opts.TTL.Seconds()),
		Scope:      opts.Scope,
		CWD:        cwd,
		SessionID:  sessionID,
	}
}

// appendApprovedLocked writes to approved.jsonl + audit.jsonl. Caller holds lock.
func (s *Store) appendApprovedLocked(entry ApprovedEntry) error {
	if err := appendJSONL(s.approvedPath(), entry); err != nil {
		return err
	}
	return appendJSONL(s.auditPath(), AuditEvent{
		Timestamp: s.now().Unix(),
		Event:     "approve",
		Hash:      entry.Hash,
		Command:   entry.Command,
		Scope:     entry.Scope,
		CWD:       entry.CWD,
		SessionID: entry.SessionID,
	})
}

// RevokeAll marks all unconsumed approvals as consumed. Returns the count
// affected.
func (s *Store) RevokeAll() (int, error) {
	count := 0
	err := s.withLock(func() error {
		entries, err := readJSONL[ApprovedEntry](s.approvedPath())
		if err != nil {
			return err
		}
		for i := range entries {
			if !entries[i].Consumed {
				entries[i].Consumed = true
				count++
			}
		}
		if count == 0 {
			return nil
		}
		if err := writeJSONLAtomic(s.approvedPath(), entries); err != nil {
			return err
		}
		return appendJSONL(s.auditPath(), AuditEvent{
			Timestamp: s.now().Unix(),
			Event:     "revoke",
			Note:      fmt.Sprintf("revoke-all: %d entries", count),
		})
	})
	return count, err
}

// CheckApproved consumes the first live, matching approval for (command, cwd,
// sessionID). Returns (true, entry, nil) if the command may proceed; the
// entry is a copy of the consumed approval for audit / hint composition.
//
// The check ignores the caller's normalized command form: callers pass the
// raw command (as received in tool_input), and CheckApproved runs Normalize
// itself so ineligible commands (dynamic) return (false, nil, nil) safely.
func (s *Store) CheckApproved(command, cwd, sessionID string) (bool, *ApprovedEntry, error) {
	hash, _, err := Normalize(command)
	if err != nil {
		// Not eligible for approval — treat as "no approval available".
		// The caller distinguishes downgrade behavior via its own Normalize call.
		return false, nil, nil
	}
	var consumed *ApprovedEntry
	err = s.withLock(func() error {
		entries, err := readJSONL[ApprovedEntry](s.approvedPath())
		if err != nil {
			return err
		}
		now := s.now().Unix()
		changed := false
		for i := range entries {
			e := &entries[i]
			if e.Consumed {
				continue
			}
			if e.Hash != hash {
				continue
			}
			expiresAt := e.ApprovedAt + e.TTLSeconds
			if now > expiresAt {
				// expired — leave marker for audit, purge later
				continue
			}
			if e.Scope == ScopeSession {
				if e.SessionID != "" && e.SessionID != sessionID {
					continue
				}
				if e.CWD != "" && e.CWD != cwd {
					continue
				}
			}
			// consume
			e.Consumed = true
			changed = true
			copyE := *e
			consumed = &copyE
			break
		}
		if !changed {
			return nil
		}
		if err := writeJSONLAtomic(s.approvedPath(), entries); err != nil {
			return err
		}
		return appendJSONL(s.auditPath(), AuditEvent{
			Timestamp: s.now().Unix(),
			Event:     "consume",
			Hash:      consumed.Hash,
			Command:   consumed.Command,
			Scope:     consumed.Scope,
			CWD:       cwd,
			SessionID: sessionID,
		})
	})
	if err != nil {
		return false, nil, err
	}
	if consumed == nil {
		return false, nil, nil
	}
	return true, consumed, nil
}

// Purge trims expired and consumed entries from approved.jsonl, and pending
// entries older than pendingMaxAge from pending.jsonl. Best-effort — the
// caller should not block on it. Returns the counts (approvedRemoved,
// pendingRemoved).
func (s *Store) Purge(pendingMaxAge time.Duration) (int, int, error) {
	var approvedRemoved, pendingRemoved int
	err := s.withLock(func() error {
		now := s.now().Unix()

		approved, err := readJSONL[ApprovedEntry](s.approvedPath())
		if err != nil {
			return err
		}
		kept := approved[:0]
		for _, e := range approved {
			expired := now > e.ApprovedAt+e.TTLSeconds
			if e.Consumed || expired {
				approvedRemoved++
				continue
			}
			kept = append(kept, e)
		}
		if approvedRemoved > 0 {
			if err := writeJSONLAtomic(s.approvedPath(), kept); err != nil {
				return err
			}
		}

		if pendingMaxAge <= 0 {
			return nil
		}
		pending, err := readJSONL[PendingEntry](s.pendingPath())
		if err != nil {
			return err
		}
		cutoff := now - int64(pendingMaxAge.Seconds())
		keptP := pending[:0]
		for _, e := range pending {
			if e.Timestamp < cutoff {
				pendingRemoved++
				continue
			}
			keptP = append(keptP, e)
		}
		if pendingRemoved > 0 {
			if err := writeJSONLAtomic(s.pendingPath(), keptP); err != nil {
				return err
			}
		}
		return nil
	})
	return approvedRemoved, pendingRemoved, err
}

