package report

import (
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/docker/go-units"
)

var defaultFuncMap = template.FuncMap{
	"ellipsis": func(s string, length int) string {
		if len(s) > length {
			return s[:length-3] + "..."
		}
		return s
	},
	"humanDuration": func(t int64) string {
		return units.HumanDuration(time.Since(time.Unix(t, 0))) + " ago"
	},
	"humanDurationFromTime": func(t time.Time) string {
		return units.HumanDuration(time.Since(t)) + " ago"
	},
	"humanSize": func(sz int64) string {
		s := units.HumanSizeWithPrecision(float64(sz), 3)
		i := strings.LastIndexFunc(s, unicode.IsNumber)
		return s[:i+1] + " " + s[i+1:]
	},
	"join":  strings.Join,
	"lower": strings.ToLower,
	"rfc3339": func(t int64) string {
		return time.Unix(t, 0).Format(time.RFC3339)
	},
	"replace": strings.Replace,
	"split":   strings.Split,
	"title":   strings.Title,
	"upper":   strings.ToUpper,
	// TODO: Remove after Go 1.14 port
	"slice": func(s string, i, j int) string {
		if i > j || len(s) < i {
			return s
		}
		if len(s) < j {
			return s[i:]
		}
		return s[i:j]
	},
}

func ReportHeader(columns ...string) []byte {
	hdr := make([]string, len(columns))
	for i, h := range columns {
		hdr[i] = strings.ToUpper(h)
	}
	return []byte(strings.Join(hdr, "\t") + "\n")
}

func AppendFuncMap(funcMap template.FuncMap) template.FuncMap {
	merged := PodmanTemplateFuncs()
	for k, v := range funcMap {
		merged[k] = v
	}
	return merged
}

func PodmanTemplateFuncs() template.FuncMap {
	merged := make(template.FuncMap)
	for k, v := range defaultFuncMap {
		merged[k] = v
	}
	return merged
}
