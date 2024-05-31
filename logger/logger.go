package logger

import (
	"io"
	"log"
	"os"
)

type Level int

const (
	InfoLevel  Level = 0x1
	DebugLevel Level = 0x2
	ErrorLevel Level = 0x4
	WarnLevel  Level = 0x8
)

const (
	TestLevel   Level = InfoLevel | DebugLevel | ErrorLevel | WarnLevel
	DeployLevel Level = InfoLevel | ErrorLevel | WarnLevel
)

const LevelNum = 4

type Logger interface {
	Printf(string, ...any)
	Println(...any)
}

var (
	infoLog  = log.New(os.Stdout, "[INFO] ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	debugLog = log.New(os.Stdout, "[DUBUG] ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	errorLog = log.New(os.Stdout, "[ERROR] ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	warnLog  = log.New(os.Stdout, "[WARN] ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	out      = []io.Writer{os.Stdout, os.Stdout, os.Stdout, os.Stdout}
	logs     = []*log.Logger{infoLog, debugLog, errorLog, warnLog}
)

func SetLevel(level Level) {
	for i := 0; i < LevelNum; i++ {
		if ((level >> i) & 1) == 1 {
			logs[i].SetOutput(out[i])
		} else {
			logs[i].SetOutput(io.Discard)
		}
	}
}

func SetOutput(level Level, w io.Writer) {
	for i := 0; i < LevelNum; i++ {
		if ((level >> i) & 1) == 1 {
			logs[i].SetOutput(w)
			out[i] = w
		}
	}
}

var (
	Info  Logger = infoLog
	Debug Logger = debugLog
	Error Logger = errorLog
	Warn  Logger = warnLog
)
