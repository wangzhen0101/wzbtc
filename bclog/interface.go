package bclog

type Logger interface {
	Tracef(format string, params ...interface{})
	Debugf(format string, params ...interface{})
	Infof(format string, params ...interface{})
	Warnf(format string, params ...interface{})
	Errorf(format string, params ...interface{})
	Fatalf(format string, params ...interface{})

	Trace(params ...interface{})
	Debug(params ...interface{})
	Info(params ...interface{})
	Warn(params ...interface{})
	Error(params ...interface{})
	Fatal(params ...interface{})

	Level() Level
	SetLevel(level Level)
}
