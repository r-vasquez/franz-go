package sr

// The log levels that can be used to control the verbosity of the
// SR client. The levels are ordered to mirror the kgo leveling.
const (
	// logLevelNone disables logging.
	logLevelNone int8 = iota
	// logLevelError logs all errors.
	logLevelError
	// logLevelWarn logs all warnings, such as request failures.
	logLevelWarn
	// logLevelInfo logs informational messages, such as requests. This is
	// usually the default log level.
	logLevelInfo
	// logLevelDebug logs verbose information, and is usually not used in
	// production.
	logLevelDebug
)
