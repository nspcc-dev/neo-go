package options

import "go.uber.org/zap/zapcore"

// FilteringCore is custom implementation of zapcore.Core that allows to filter
// log entries using custom filtering function.
type FilteringCore struct {
	zapcore.Core
	filter FilterFunc
}

// FilterFunc is the filter function that is called to check whether the given
// entry together with the associated fields is to be written to a core or not.
type FilterFunc func(zapcore.Entry) bool

// NewFilteringCore returns a core middleware that uses the given filter function
// to decide whether to log this message or not.
func NewFilteringCore(next zapcore.Core, filter FilterFunc) zapcore.Core {
	return &FilteringCore{next, filter}
}

// Check implements zapcore.Core interface and performs log entries filtering.
func (c *FilteringCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.filter(e) {
		return c.Core.Check(e, ce)
	}
	return ce
}
