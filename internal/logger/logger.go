package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
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

// ParseLevel parses a level string into a slog.Level.
// Accepts "debug", "verbose", "info", "warn", "warning", "error" (case-insensitive).
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug", "verbose":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level: %q", s)
	}
}

// ConsoleHandler implements slog.Handler for readable, colored terminal output.
type ConsoleHandler struct {
	mu          sync.Mutex
	out         io.Writer
	level       slog.Leveler
	enableColor bool
	attrs       []slog.Attr
	groups      []string
}

// NewConsoleHandler creates a new ConsoleHandler.
func NewConsoleHandler(out io.Writer, level slog.Leveler, enableColor bool) *ConsoleHandler {
	if out == nil {
		out = os.Stdout
	}
	if level == nil {
		level = slog.LevelInfo
	}
	return &ConsoleHandler{
		out:         out,
		level:       level,
		enableColor: enableColor,
	}
}

// Enabled reports whether the handler emits log records at the given level.
func (h *ConsoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.level != nil {
		minLevel = h.level.Level()
	}
	return level >= minLevel
}

// Handle formats and writes the slog.Record to the output writer.
func (h *ConsoleHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	ts := r.Time.Format("2006-01-02 15:04:05")
	if r.Time.IsZero() {
		ts = time.Now().Format("2006-01-02 15:04:05")
	}

	var levelTag, color string
	switch {
	case r.Level >= slog.LevelError:
		levelTag = "[ERROR]"
		color = ColorRed
	case r.Level >= slog.LevelWarn:
		levelTag = "[WARN]"
		color = ColorYellow
	case r.Level >= slog.LevelInfo:
		levelTag = "[INFO]"
		color = ColorCyan
	default:
		levelTag = "[DEBUG]"
		color = ColorGray
	}

	var sb strings.Builder
	if h.enableColor {
		sb.WriteString(ColorGray)
		sb.WriteString(ts)
		sb.WriteString(ColorReset)
		sb.WriteString(" ")
		sb.WriteString(color)
		sb.WriteString(levelTag)
		sb.WriteString(ColorReset)
		sb.WriteString(" ")
		sb.WriteString(r.Message)
	} else {
		sb.WriteString(ts)
		sb.WriteString(" ")
		sb.WriteString(levelTag)
		sb.WriteString(" ")
		sb.WriteString(r.Message)
	}

	for _, a := range h.attrs {
		appendAttr(&sb, a, h.groups, h.enableColor)
	}

	r.Attrs(func(a slog.Attr) bool {
		appendAttr(&sb, a, h.groups, h.enableColor)
		return true
	})

	sb.WriteString("\n")
	_, err := fmt.Fprint(h.out, sb.String())
	return err
}

func appendAttr(sb *strings.Builder, a slog.Attr, groups []string, enableColor bool) {
	if a.Equal(slog.Attr{}) {
		return
	}
	sb.WriteString(" ")
	key := a.Key
	if len(groups) > 0 {
		key = strings.Join(groups, ".") + "." + key
	}
	if enableColor {
		sb.WriteString(ColorCyan)
		sb.WriteString(key)
		sb.WriteString(ColorReset)
		sb.WriteString("=")
		sb.WriteString(fmt.Sprint(a.Value.Any()))
	} else {
		sb.WriteString(key)
		sb.WriteString("=")
		sb.WriteString(fmt.Sprint(a.Value.Any()))
	}
}

// WithAttrs returns a new Handler with the given attributes added.
func (h *ConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h
	h2.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &h2
}

// WithGroup returns a new Handler with the given group appended.
func (h *ConsoleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := *h
	h2.groups = append(append([]string(nil), h.groups...), name)
	return &h2
}

// Setup configures and sets the default slog logger based on level, format, and color preference.
func Setup(out io.Writer, level slog.Leveler, format string, enableColor bool) *slog.Logger {
	if out == nil {
		out = os.Stdout
	}
	var handler slog.Handler
	if strings.ToLower(strings.TrimSpace(format)) == "json" {
		handler = slog.NewJSONHandler(out, &slog.HandlerOptions{Level: level})
	} else {
		handler = NewConsoleHandler(out, level, enableColor)
	}
	l := slog.New(handler)
	slog.SetDefault(l)
	return l
}

// ConfigItem represents a single key-value configuration setting.
type ConfigItem struct {
	Key       string
	Value     any
	Sensitive bool
}

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
func FormatConfig(title string, items []ConfigItem, enableColor bool) string {
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
		if enableColor {
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

		if enableColor {
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

// PrintConfig formats and writes configuration items to stdout.
func PrintConfig(title string, items []ConfigItem) {
	out := FormatConfig(title, items, true)
	fmt.Print(out)
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
