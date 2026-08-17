package logs

// Logger interface represents the logs methods you want to use in your application.
type Logger interface {
	Info(args ...interface{})
	Error(args ...interface{})
	WithFields(fields map[string]interface{}) Logger
}
