package gormxray

// Option is a configuration option for NewPlugin.
type Option func(*PluginConfig)

// WithExcludeQueryVars controls whether query variables are included in the metadata.
func WithExcludeQueryVars(exclude bool) Option {
	return func(pc *PluginConfig) {
		pc.ExcludeQueryVars = exclude
	}
}

// WithQueryFormatter allows providing a custom function to format queries before adding them as metadata.
func WithQueryFormatter(formatter func(string) string) Option {
	return func(pc *PluginConfig) {
		pc.QueryFormatter = formatter
	}
}
