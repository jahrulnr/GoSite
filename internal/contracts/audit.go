package contracts

import (
	"context"
	"time"
)

// AuditEntry is a single audit log record for sensitive mutations.
type AuditEntry struct {
	Timestamp    time.Time
	UserEmail    string
	Action       string
	ResourceType string
	ResourceID   string
	Domain       string
	Status       string
	Message      string
	MetaJSON     string
}

// AuditWriter persists audit events for Splunk Lite queries.
type AuditWriter interface {
	Write(ctx context.Context, entry AuditEntry) error
}
