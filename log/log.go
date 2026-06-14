package log

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var defaultLevel = slog.LevelDebug

func SetLevel(lev slog.Level) {
	defaultLevel = lev
}

func SetDebug(debug bool) {
	if debug {
		defaultLevel = slog.LevelDebug
	} else {
		defaultLevel = slog.LevelInfo
	}
}

var (
	textColors = map[slog.Level]string{
		slog.Level(slog.LevelError + 4): "\033[97;41m",
		slog.LevelError:                 "\u001b[97;101m",
		slog.LevelWarn:                  "\u001b[93m",
		slog.LevelInfo:                  "\u001b[32m",
		slog.LevelDebug:                 "\u001b[36m",
		slog.Level(slog.LevelDebug - 4): "\u001b[37m",
	}
)

var (
	logger = new(atomic.Value)
	lck    = new(sync.Mutex)
)

type coloredHandler struct {
}

func (h *coloredHandler) Enabled(x context.Context, lev slog.Level) bool {
	return lev >= defaultLevel
}

func (h *coloredHandler) Handle(x context.Context, item slog.Record) error {
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "%s", item.Time.Format(time.RFC3339))

	fi := getFuncInfo(item.PC)
	fmt.Fprintf(buf, " <%s/%s:%d>", fi.mod, fi.fileName, fi.line)
	// if src := item.Source(); src != nil {
	// 	fname := filepath.Base(src.File)
	// 	fmt.Fprintf(buf, " <%s:%d,%s()> ", fname, src.Line, src.Function)
	// }

	fmt.Fprintf(buf, " [")
	if c, found := textColors[item.Level]; found {
		fmt.Fprintf(buf, "%s%s\u001b[0m", c, item.Level)
	} else {
		fmt.Fprintf(buf, "%s", item.Level.String())
	}
	fmt.Fprintf(buf, "] ")

	fmt.Fprintf(buf, "%s", item.Message)

	item.Attrs(func(attr slog.Attr) bool {
		switch attr.Key {
		case slog.TimeKey:
		case slog.MessageKey:
		case slog.SourceKey:
		case slog.LevelKey:
		default:
			fmt.Fprintf(
				buf,
				" %s\x1b[37m=\x1b[0m%s",
				attr.Key,
				attr.Value,
			)
		}
		return true
	})

	fmt.Fprintln(os.Stdout, string(buf.Bytes()))
	return nil
}

func (h *coloredHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *coloredHandler) WithGroup(g string) slog.Handler {
	return h
}

func Logger() *slog.Logger {
	if v := logger.Load(); v != nil {
		return v.(*slog.Logger)
	}
	lck.Lock()
	defer lck.Unlock()

	l := slog.New(&coloredHandler{})

	logger.Store(l)

	return l
}

// runtime frames

type funcInfo struct {
	mod      string
	fileName string
	line     int
}

func getFuncInfo(pc0 uintptr) *funcInfo {
	fi := &funcInfo{}

	_, fileThis, _, ok := runtime.Caller(0)
	if !ok {
		return fi
	}
	folder := filepath.Dir(fileThis)

	pc := make([]uintptr, 64)
	n := runtime.Callers(0, pc)
	if n == 0 {
		return fi
	}
	pc = pc[:n]
	frames := runtime.CallersFrames(pc)

	prevFolderIsLog := false

	for {
		frame, more := frames.Next()
		frameFolder := filepath.Dir(frame.File)

		isLogDir := frameFolder == folder
		if !isLogDir && prevFolderIsLog {
			funcName := frame.Function
			parts := strings.Split(funcName, "/")
			lastPart := parts[len(parts)-1]
			lastPartArr := strings.Split(lastPart, ".")
			modName := lastPartArr[0]
			parts = parts[:len(parts)-1]
			parts = append(parts, modName)
			modPath := strings.Join(parts, "/")
			fi.mod = modPath
			fi.fileName = filepath.Base(frame.File)
			fi.line = frame.Line
		}
		prevFolderIsLog = isLogDir
		if !more {
			break
		}
	}
	return fi
}

// convenient functions

func Debug(msg string, args ...any) {
	Logger().Debug(msg, args...)
}

func Info(msg string, args ...any) {
	Logger().Info(msg, args...)
}

func Warn(msg string, args ...any) {
	Logger().Warn(msg, args...)
}

func Error(msg string, args ...any) {
	Logger().Error(msg, args...)
}
