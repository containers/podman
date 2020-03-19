package report

import (
	"strings"
	"text/template"
	"time"

	"github.com/docker/go-units"
)

var defaultFuncMap = template.FuncMap{
	"ellipsis": func(s string, length int) string {
		if len(s) > length {
			return s[:length-3] + "..."
		}
		return s
	},
	// TODO: Remove on Go 1.14 port
	"slice": func(s string, i, j int) string {
		if i > j || len(s) < i {
			return s
		}
		if len(s) < j {
			return s[i:]
		}
		return s[i:j]
	},
	"toRFC3339": func(t int64) string {
		return time.Unix(t, 0).Format(time.RFC3339)
	},
	"toHumanDuration": func(t int64) string {
		return units.HumanDuration(time.Since(time.Unix(t, 0))) + " ago"
	},
	"toHumanSize": func(sz int64) string {
		return units.HumanSize(float64(sz))
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
