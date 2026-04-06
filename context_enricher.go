package unilog

import "context"

type enrichContextKey string

const (
	userIDKey    enrichContextKey = "unilog_user_id"
	tenantIDKey  enrichContextKey = "unilog_tenant_id"
	sessionIDKey enrichContextKey = "unilog_session_id"
)

func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func WithTenantID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, tenantIDKey, id)
}

func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey, id)
}

func UserIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

func TenantIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(tenantIDKey).(string)
	return v
}

func SessionIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(sessionIDKey).(string)
	return v
}

type ContextEnricherOptions struct {
	IncludeUserID    bool
	IncludeTenantID  bool
	IncludeSessionID bool

	UserFieldName    string
	TenantFieldName  string
	SessionFieldName string
}

type ContextEnricher struct {
	opts ContextEnricherOptions
}

func NewContextEnricher(opts ContextEnricherOptions) *ContextEnricher {
	if opts.UserFieldName == "" {
		opts.UserFieldName = "user_id"
	}
	if opts.TenantFieldName == "" {
		opts.TenantFieldName = "tenant_id"
	}
	if opts.SessionFieldName == "" {
		opts.SessionFieldName = "session_id"
	}

	// Default to all enabled unless explicitly set false by custom bulder usage
	if !opts.IncludeUserID && !opts.IncludeTenantID && !opts.IncludeSessionID {
		opts.IncludeUserID = true
		opts.IncludeSessionID = true
		opts.IncludeTenantID = true
	}

	return &ContextEnricher{opts: opts}
}

func (p *ContextEnricher) Name() string {
	return "context_enricher"
}

func (p *ContextEnricher) Process(ctx context.Context, event *Event) error {
	if event == nil || ctx == nil {
		return nil
	}
	if p.opts.IncludeUserID {
		if v := UserIDFromContext(ctx); v != "" && !fieldExists(event.Fields, p.opts.UserFieldName) {
			event.Fields = append(event.Fields, String(p.opts.UserFieldName, v))
		}
	}

	if p.opts.IncludeTenantID {
		if v := TenantIDFromContext(ctx); v != "" && !fieldExists(event.Fields, p.opts.TenantFieldName) {
			event.Fields = append(event.Fields, String(p.opts.TenantFieldName, v))
		}
	}

	if p.opts.IncludeSessionID {
		if v := SessionIDFromContext(ctx); v != "" && !fieldExists(event.Fields, p.opts.SessionFieldName) {
			event.Fields = append(event.Fields, String(p.opts.SessionFieldName, v))
		}
	}
	return nil
}

func fieldExists(fields []Field, key string) bool {
	for _, f := range fields {
		if f.Key == key {
			return true
		}
	}
	return false
}
