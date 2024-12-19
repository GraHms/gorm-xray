package gormxray

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/aws/aws-xray-sdk-go/xray"
	"io"
	"log"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

// Regular expressions for parsing SQL statements.
var (
	firstWordRegex   = regexp.MustCompile(`^\w+`)
	cCommentRegex    = regexp.MustCompile(`(?is)/\*.*?\*/`)
	lineCommentRegex = regexp.MustCompile(`(?im)(?:--|#).*?$`)
	sqlPrefixRegex   = regexp.MustCompile(`^[\s;]*`)
)

// PluginConfig allows customization of the plugin's behavior.
type PluginConfig struct {
	ExcludeQueryVars bool
	ExcludeMetrics   bool
	QueryFormatter   func(string) string
}

// Plugin implements gorm.Plugin to integrate AWS X-Ray gormxray into GORM operations.
type Plugin struct {
	excludeQueryVars bool
	excludeMetrics   bool
	queryFormatter   func(string) string
}

// NewPlugin creates a new X-Ray plugin for GORM using functional options.
func NewPlugin(opts ...Option) gorm.Plugin {
	cfg := &PluginConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return &Plugin{
		excludeQueryVars: cfg.ExcludeQueryVars,
		excludeMetrics:   cfg.ExcludeMetrics,
		queryFormatter:   cfg.QueryFormatter,
	}
}

// Name returns the plugin's name.
func (p Plugin) Name() string {
	return "xraytracing"
}

type gormHookFunc func(tx *gorm.DB)

type gormRegister interface {
	Register(name string, fn func(*gorm.DB)) error
}

// Initialize attaches the plugin's hooks into the GORM lifecycle.
func (p Plugin) Initialize(db *gorm.DB) (err error) {
	cb := db.Callback()

	hooks := []struct {
		callback gormRegister
		hook     gormHookFunc
		name     string
	}{
		{cb.Create().Before("gorm:create"), p.before("gorm.Create"), "before:create"},
		{cb.Create().After("gorm:create"), p.after(), "after:create"},
		{cb.Query().Before("gorm:query"), p.before("gorm.Query"), "before:select"},
		{cb.Query().After("gorm:query"), p.after(), "after:select"},
		{cb.Delete().Before("gorm:delete"), p.before("gorm.Delete"), "before:delete"},
		{cb.Delete().After("gorm:delete"), p.after(), "after:delete"},
		{cb.Update().Before("gorm:update"), p.before("gorm.Update"), "before:update"},
		{cb.Update().After("gorm:update"), p.after(), "after:update"},
		{cb.Row().Before("gorm:row"), p.before("gorm.Row"), "before:row"},
		{cb.Row().After("gorm:row"), p.after(), "after:row"},
		{cb.Raw().Before("gorm:raw"), p.before("gorm.Raw"), "before:raw"},
		{cb.Raw().After("gorm:raw"), p.after(), "after:raw"},
	}

	var firstErr error
	for _, h := range hooks {
		if err := h.callback.Register("xray:"+h.name, h.hook); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("callback register %s failed: %w", h.name, err)
			log.Printf("[ERROR] Could not register callback %s: %v", h.name, err)
		}
	}

	return firstErr
}

// before hook starts an X-Ray subsegment before the query is executed.
func (p *Plugin) before(spanName string) gormHookFunc {
	return func(tx *gorm.DB) {
		// Ensure the context has an active parent segment
		if xray.GetSegment(tx.Statement.Context) == nil {
			tx.Statement.Context, _ = xray.BeginSegment(tx.Statement.Context, "FallbackParent")
		}
		ctx, seg := xray.BeginSubsegment(tx.Statement.Context, spanName)
		tx.Statement.Context = ctx
		tx.InstanceSet("xray_subsegment", seg)
	}
}

// after hook closes the X-Ray subsegment after the query is executed and adds metadata.
func (p *Plugin) after() gormHookFunc {
	return func(tx *gorm.DB) {
		val, ok := tx.InstanceGet("xray_subsegment")
		if !ok {
			return
		}

		subSegment, ok := val.(*xray.Segment)
		if !ok || subSegment == nil {
			return
		}
		defer subSegment.Close(nil)

		var query string
		if p.excludeQueryVars {
			query = tx.Statement.SQL.String()
		} else {
			query = tx.Dialector.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...)
		}

		formatQuery := p.formatQuery(query)
		subSegment.AddMetadata("db.query", formatQuery)
		subSegment.AddMetadata("db.operation", dbOperation(formatQuery))
		if tx.Statement.Table != "" {
			subSegment.AddMetadata("db.table", tx.Statement.Table)
		}
		if tx.Statement.RowsAffected != -1 {
			subSegment.AddMetadata("db.rows.affected", tx.Statement.RowsAffected)
		}

		// Record errors if any
		switch tx.Error {
		case nil,
			gorm.ErrRecordNotFound,
			driver.ErrSkip,
			io.EOF,
			sql.ErrNoRows:
			// These are considered non-critical "errors" for X-Ray.
		default:
			subSegment.AddError(tx.Error)
		}
	}
}

// formatQuery applies a custom query formatter if provided.
func (p *Plugin) formatQuery(query string) string {
	if p.queryFormatter != nil {
		return p.queryFormatter(query)
	}
	return query
}

// dbOperation extracts the first SQL keyword from the query to identify the operation (e.g., SELECT, INSERT).
func dbOperation(query string) string {
	s := cCommentRegex.ReplaceAllString(query, "")
	s = lineCommentRegex.ReplaceAllString(s, "")
	s = sqlPrefixRegex.ReplaceAllString(s, "")
	return strings.ToLower(firstWordRegex.FindString(s))
}
