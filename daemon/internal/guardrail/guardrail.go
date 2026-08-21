package guardrail

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Decision is what SafeExec instructs the caller to do.
type Decision string

const (
	DecisionExecute  Decision = "execute"  // run now (READ_ONLY)
	DecisionConfirm  Decision = "confirm"  // ask user first (L1/L2)
	DecisionDeny     Decision = "deny"     // blacklist / policy hit
	DecisionCooldown Decision = "cooldown" // rate limited
)

// Request is a single exec proposal.
type Request struct {
	ID       string // correlation id
	ClientID string // ws session id for rate limiting
	Command  string
	Workdir  string
	DryRun   bool
}

// Outcome communicates the policy result.
type Outcome struct {
	Decision Decision
	Class    Classification
	Message  string
}

var (
	ErrPendingExists   = errors.New("guardrail: request already pending")
	ErrNotPending      = errors.New("guardrail: no pending request with that id")
	ErrConfirmExpired  = errors.New("guardrail: pending request expired")
)

// ConfirmTTL is how long a pending L1/L2 command waits for user approval.
const ConfirmTTL = 30 * time.Second

// SafeExec is the full policy engine.
type SafeExec struct {
	blacklist *Blacklist
	limiter   *RateLimiter
	audit     *AuditStore
	user      string

	mu      sync.Mutex
	pending map[string]*pendingReq
}

type pendingReq struct {
	req    Request
	class  Classification
	expires time.Time
}

// New wires the engine together. audit may be nil (tests).
func New(audit *AuditStore) *SafeExec {
	return &SafeExec{
		blacklist: NewBlacklist(),
		limiter:   NewRateLimiter(DefaultRateLimit()),
		audit:     audit,
		pending:   make(map[string]*pendingReq),
		user:      currentUser(),
	}
}

// NewWithRateLimit allows custom limiter tuning (tests / hardening).
func NewWithRateLimit(cfg RateLimiterConfig) *SafeExec {
	return &SafeExec{
		blacklist: NewBlacklist(),
		limiter:   NewRateLimiter(cfg),
		pending:   make(map[string]*pendingReq),
		user:      currentUser(),
	}
}

// currentUser resolves the executing user once for audit rows.
func currentUser() string {
	u := osUser()
	if u == "" {
		return "unknown"
	}
	return u
}

// Evaluate runs the full pipeline on an incoming request:
// rate limit -> blacklist -> classify -> dry-run/confirm/execute decision.
func (s *SafeExec) Evaluate(req Request) Outcome {
	// 1. rate limiting / anomaly control
	if !s.limiter.Allow(req.ClientID) {
		s.auditDenied(req, LevelReadOnly, "rate limited")
		return Outcome{Decision: DecisionCooldown, Message: "too many requests; cooldown active"}
	}

	// 2+3. blacklist + classification (chain-aware)
	class := s.blacklist.Classify(req.Command)

	if class.Denied {
		s.auditDenied(req, class.Level, class.Reason)
		return Outcome{
			Decision: DecisionDeny,
			Class:    class,
			Message: fmt.Sprintf("SECURITY ALERT: %s. Execution aborted by Local Guardrail Engine.", class.Reason),
		}
	}

	// 4. dry-run short-circuits before confirmation
	if req.DryRun {
		s.auditDryRun(req, class.Level)
		return Outcome{Decision: DecisionExecute, Class: class, Message: "dry-run preview"}
	}

	switch class.Level {
	case LevelReadOnly:
		s.auditAuto(req, class)
		return Outcome{Decision: DecisionExecute, Class: class}
	case LevelWorkspaceWrite, LevelFullSystem:
		if err := s.stageForConfirmation(req, class); err != nil {
			return Outcome{Decision: DecisionDeny, Class: class, Message: err.Error()}
		}
		return Outcome{Decision: DecisionConfirm, Class: class}
	default:
		return Outcome{Decision: DecisionDeny, Class: class, Message: "unknown classification"}
	}
}

// stageForConfirmation registers the request as awaiting user approval.
func (s *SafeExec) stageForConfirmation(req Request, class Classification) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.pending[req.ID]; exists {
		return ErrPendingExists
	}
	s.pending[req.ID] = &pendingReq{
		req:    req,
		class:  class,
		expires: time.Now().Add(ConfirmTTL),
	}
	return nil
}

// Approve validates user approval for a pending request. It returns the
// stored request so the caller can execute exactly what was staged.
func (s *SafeExec) Approve(requestID string, approve bool) (*Request, Classification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.pending[requestID]
	if !ok {
		return nil, Classification{}, ErrNotPending
	}
	delete(s.pending, requestID)

	if time.Now().After(p.expires) {
		s.auditRow(p.req, p.class, false, "denied", "expired", nil)
		return nil, p.class, ErrConfirmExpired
	}
	if !approve {
		s.auditRow(p.req, p.class, false, "denied", "denied", nil)
		return &p.req, p.class, nil
	}
	s.auditRow(p.req, p.class, false, "user", "approved", nil)
	return &p.req, p.class, nil
}

// PendingCount exposes how many requests await approval (metrics/tests).
func (s *SafeExec) PendingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

// DryRunPreview renders a simulated preview of the command without running it.
func (s *SafeExec) DryRunPreview(command string) string {
	class := s.blacklist.Classify(command)
	if class.Denied {
		return fmt.Sprintf("[DRY-RUN DENIED] %s would be rejected: %s", command, class.Reason)
	}
	return fmt.Sprintf("[DRY-RUN %s] %s", class.Level, command)
}

func (s *SafeExec) auditDenied(req Request, lvl Level, reason string) {
	s.limiter.RecordDenied(req.ClientID)
	s.auditRow(req, Classification{Level: lvl, Denied: true, Reason: reason}, true, "denied", "denied", nil)
}

func (s *SafeExec) auditAuto(req Request, class Classification) {
	s.auditRow(req, class, false, "auto", "executed", nil)
}

func (s *SafeExec) auditDryRun(req Request, lvl Level) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Append(AuditLogEntry{
		ID: newAuditID(), Timestamp: time.Now(),
		Command: req.Command, Classification: lvl.String(),
		User: s.user, Status: "dry_run",
	})
}

func (s *SafeExec) auditRow(req Request, class Classification, denied bool, approvedBy, status string, exitCode *int) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Append(AuditLogEntry{
		ID: newAuditID(), Timestamp: time.Now(),
		Command: req.Command, Classification: class.String(),
		Denied: denied, ApprovedBy: approvedBy, Status: status,
		User: s.user, ExitCode: exitCode,
	})
}

// RecordResult appends execution telemetry after the fact.
func (s *SafeExec) RecordResult(ctx context.Context, command string, lvl Level, status string, exitCode *int, dur time.Duration) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Append(AuditLogEntry{
		ID: newAuditID(), Timestamp: time.Now(),
		Command: command, Classification: lvl.String(),
		Status: status, ExitCode: exitCode,
		DurationMS: dur.Milliseconds(), User: s.user,
	})
}