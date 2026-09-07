package logger

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

// ANSI color codes.
const (
	ColorReset  = "\033[0m"
	ColorBold   = "\033[1m"
	ColorDim    = "\033[2m"
	ColorGray   = "\033[90m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
)

// ConfigItem represents a single key-value configuration setting.
type ConfigItem struct {
	Key       string
	Value     any
	Sensitive bool
}

// Logger provides ANSI colored leveled logging and configuration formatting.
type Logger struct {
	mu          sync.Mutex
	out         io.Writer
	enableColor bool
}

var (
	defaultLogger = New(os.Stdout)
)

// New creates a new Logger writing to the specified writer.
func New(out io.Writer) *Logger {
	if out == nil {
		out = os.Stdout
	}
	return &Logger{
		out:         out,
		enableColor: true,
	}
}

// SetOutput changes the destination writer for the default logger.
func SetOutput(w io.Writer) {
	defaultLogger.SetOutput(w)
}

// SetColor enables or disables ANSI color output on the default logger.
func SetColor(enabled bool) {
	defaultLogger.SetColor(enabled)
}

// SetOutput changes the destination writer.
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = w
}

// SetColor enables or disables ANSI color output.
func (l *Logger) SetColor(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enableColor = enabled
}

func (l *Logger) log(levelTag, color, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	ts := time.Now().Format("2006-01-02 15:04:05")

	var line string
	if l.enableColor {
		line = fmt.Sprintf("%s%s%s %s%s%s %s\n",
			ColorGray, ts, ColorReset,
			color, levelTag, ColorReset,
			msg,
		)
	} else {
		line = fmt.Sprintf("%s %s %s\n", ts, levelTag, msg)
	}

	fmt.Fprint(l.out, line)
}

// Info logs an informational message in cyan.
func (l *Logger) Info(args ...any) {
	l.log("[INFO]", ColorCyan, fmt.Sprint(args...))
}

// Infof logs a formatted informational message in cyan.
func (l *Logger) Infof(format string, args ...any) {
	l.log("[INFO]", ColorCyan, fmt.Sprintf(format, args...))
}

// Warn logs a warning message in yellow.
func (l *Logger) Warn(args ...any) {
	l.log("[WARN]", ColorYellow, fmt.Sprint(args...))
}

// Warnf logs a formatted warning message in yellow.
func (l *Logger) Warnf(format string, args ...any) {
	l.log("[WARN]", ColorYellow, fmt.Sprintf(format, args...))
}

// Error logs an error message in red.
func (l *Logger) Error(args ...any) {
	l.log("[ERROR]", ColorRed, fmt.Sprint(args...))
}

// Errorf logs a formatted error message in red.
func (l *Logger) Errorf(format string, args ...any) {
	l.log("[ERROR]", ColorRed, fmt.Sprintf(format, args...))
}

// Debug logs a debug message in dim/gray.
func (l *Logger) Debug(args ...any) {
	l.log("[DEBUG]", ColorGray, fmt.Sprint(args...))
}

// Debugf logs a formatted debug message in dim/gray.
func (l *Logger) Debugf(format string, args ...any) {
	l.log("[DEBUG]", ColorGray, fmt.Sprintf(format, args...))
}

// Package-level functions delegating to defaultLogger.

func Info(args ...any)                  { defaultLogger.Info(args...) }
func Infof(format string, args ...any)  { defaultLogger.Infof(format, args...) }
func Warn(args ...any)                  { defaultLogger.Warn(args...) }
func Warnf(format string, args ...any)  { defaultLogger.Warnf(format, args...) }
func Error(args ...any)                 { defaultLogger.Error(args...) }
func Errorf(format string, args ...any) { defaultLogger.Errorf(format, args...) }
func Debug(args ...any)                 { defaultLogger.Debug(args...) }
func Debugf(format string, args ...any) { defaultLogger.Debugf(format, args...) }

// IsSensitiveKey checks if a key name suggests sensitive content.
func IsSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, sub := range []string{"token", "secret", "password", "key", "auth", "credential"} {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

// MaskValue formats sensitive values as "[configured: N chars]" or "[disabled / open]".
func MaskValue(val any) (string, bool) {
	if val == nil {
		return "[disabled / open]", false
	}

	var str string
	switch v := val.(type) {
	case string:
		str = v
	case *string:
		if v == nil {
			return "[disabled / open]", false
		}
		str = *v
	case []byte:
		str = string(v)
	default:
		str = fmt.Sprintf("%v", val)
	}

	if str == "" {
		return "[disabled / open]", false
	}
	return fmt.Sprintf("[configured: %d chars]", len(str)), true
}

// FormatConfig returns a clean ANSI aligned plain-text representation of configuration settings.
func (l *Logger) FormatConfig(title string, items []ConfigItem) string {
	if len(items) == 0 {
		return ""
	}

	maxKeyLen := 0
	for _, it := range items {
		if len(it.Key) > maxKeyLen {
			maxKeyLen = len(it.Key)
		}
	}

	var sb strings.Builder
	if title != "" {
		if l.enableColor {
			sb.WriteString(fmt.Sprintf("%s%s%s\n", ColorBold, title, ColorReset))
		} else {
			sb.WriteString(title + "\n")
		}
	}

	for _, it := range items {
		key := it.Key
		isSens := it.Sensitive || IsSensitiveKey(key)

		var valStr string
		var isConfigured bool
		if isSens {
			valStr, isConfigured = MaskValue(it.Value)
		} else {
			valStr = fmt.Sprintf("%v", it.Value)
		}

		if l.enableColor {
			var coloredVal string
			if isSens {
				if isConfigured {
					coloredVal = fmt.Sprintf("%s%s%s", ColorYellow, valStr, ColorReset)
				} else {
					coloredVal = fmt.Sprintf("%s%s%s", ColorGray, valStr, ColorReset)
				}
			} else {
				coloredVal = fmt.Sprintf("%s%s%s", ColorGreen, valStr, ColorReset)
			}

			sb.WriteString(fmt.Sprintf("  %s%-*s%s %s:%s %s\n",
				ColorCyan, maxKeyLen, key, ColorReset,
				ColorGray, ColorReset,
				coloredVal,
			))
		} else {
			sb.WriteString(fmt.Sprintf("  %-*s : %s\n", maxKeyLen, key, valStr))
		}
	}

	return sb.String()
}

// PrintConfig formats and writes configuration items to the logger output.
func (l *Logger) PrintConfig(args ...any) {
	title := "Configuration:"
	var items []ConfigItem

	if len(args) == 1 {
		switch v := args[0].(type) {
		case []ConfigItem:
			items = v
		case map[string]any:
			items = MapToConfigItems(v)
		default:
			items = StructToConfigItems(v)
		}
	} else if len(args) >= 2 {
		if t, ok := args[0].(string); ok {
			title = t
		}
		switch v := args[1].(type) {
		case []ConfigItem:
			items = v
		case map[string]any:
			items = MapToConfigItems(v)
		default:
			items = StructToConfigItems(v)
		}
	}

	out := l.FormatConfig(title, items)
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprint(l.out, out)
}

// Package-level FormatConfig delegating to defaultLogger.
func FormatConfig(title string, items []ConfigItem) string {
	return defaultLogger.FormatConfig(title, items)
}

// Package-level PrintConfig delegating to defaultLogger.
func PrintConfig(args ...any) {
	defaultLogger.PrintConfig(args...)
}

// MapToConfigItems converts a map to sorted ConfigItem slice.
func MapToConfigItems(m map[string]any) []ConfigItem {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	items := make([]ConfigItem, 0, len(keys))
	for _, k := range keys {
		items = append(items, ConfigItem{
			Key:       k,
			Value:     m[k],
			Sensitive: IsSensitiveKey(k),
		})
	}
	return items
}

// StructToConfigItems converts an exported struct or pointer to struct into ConfigItem slice.
func StructToConfigItems(v any) []ConfigItem {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return []ConfigItem{{Key: "Value", Value: fmt.Sprintf("%v", v)}}
	}

	typ := val.Type()
	var items []ConfigItem
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		key := field.Name
		tag := field.Tag.Get("config")
		if tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] == "-" {
				continue
			}
			if parts[0] != "" {
				key = parts[0]
			}
		}

		sensitive := IsSensitiveKey(field.Name) || field.Tag.Get("sensitive") == "true" || field.Tag.Get("mask") == "true"

		items = append(items, ConfigItem{
			Key:       key,
			Value:     val.Field(i).Interface(),
			Sensitive: sensitive,
		})
	}

	return items
}
