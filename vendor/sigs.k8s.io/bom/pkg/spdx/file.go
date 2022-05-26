/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package spdx

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var fileTemplate = `{{ if .Name }}FileName: {{ .Name }}
{{ end -}}
{{ if .ID }}SPDXID: {{ .ID }}
{{ end -}}
{{- if .Checksum -}}
{{- range $key, $value := .Checksum -}}
{{ if . }}FileChecksum: {{ $key }}: {{ $value }}
{{ end -}}
{{- end -}}
{{- end -}}
{{- if .FileType -}}
{{- range $key, $value := .FileType -}}
{{ if . }}FileType: {{ $value }}
{{ end -}}
{{- end -}}
{{- end -}}
LicenseConcluded: {{ if .LicenseConcluded }}{{ .LicenseConcluded }}{{ else }}NOASSERTION{{ end }}
LicenseInfoInFile: {{ if .LicenseInfoInFile }}{{ .LicenseInfoInFile }}{{ else }}NOASSERTION{{ end }}
FileCopyrightText: {{ if .CopyrightText }}<text>{{ .CopyrightText }}
</text>{{ else }}NOASSERTION{{ end }}

`

// File abstracts a file contained in a package
type File struct {
	Entity
	FileType          []string
	LicenseInfoInFile string // GPL-3.0-or-later
}

func NewFile() (f *File) {
	f = &File{}
	f.Entity.Opts = &ObjectOptions{}
	return f
}

// Render renders the document fragment of a file
func (f *File) Render() (docFragment string, err error) {
	// If we have not yet checksummed the file, do it now:
	if f.Checksum == nil || len(f.Checksum) == 0 {
		if f.SourceFile != "" {
			if err := f.ReadSourceFile(f.SourceFile); err != nil {
				return "", errors.Wrap(err, "checksumming file")
			}
		} else {
			logrus.Warnf(
				"File %s does not have checksums, SBOM will not be SPDX compliant", f.ID,
			)
		}
	}
	var buf bytes.Buffer
	tmpl, err := template.New("file").Parse(fileTemplate)
	if err != nil {
		return "", errors.Wrap(err, "parsing file template")
	}

	// Run the template to verify the output.
	if err := tmpl.Execute(&buf, f); err != nil {
		return "", errors.Wrap(err, "executing spdx file template")
	}

	docFragment = buf.String()
	return docFragment, nil
}

// BuildID sets the file ID, optionally from a series of strings
func (f *File) BuildID(seeds ...string) {
	prefix := ""
	if f.Options() != nil {
		if f.Options().Prefix != "" {
			prefix = "-" + f.Options().Prefix
		}
	}
	f.Entity.BuildID(append([]string{"SPDXRef-File" + prefix}, seeds...)...)
}

func (f *File) SetEntity(e *Entity) {
	f.Entity = *e
}

// Draw renders the file data as a tree-like structure
// nolint:gocritic
func (f *File) Draw(builder *strings.Builder, o *DrawingOptions, depth int, seen *map[string]struct{}) {
	connector := connectorT
	if o.LastItem {
		connector = connectorL
	}
	fmt.Fprintf(builder, treeLines(o, depth, connector)+"%s (%s)\n", f.SPDXID(), f.Name)
}

func (f *File) ReadSourceFile(path string) error {
	if err := f.Entity.ReadSourceFile(path); err != nil {
		return err
	}
	if f.SPDXID() == "" {
		f.BuildID()
	}

	f.FileType = getFileTypes(path)

	return nil
}

func getFileTypes(path string) []string {
	fileExtension := strings.TrimLeft(filepath.Ext(path), ".")

	if fileExtension == "" {
		mineType, err := getFileContentType(path)
		if err != nil {
			return []string{"OTHER"}
		}
		splited := strings.Split(mineType, "/")

		fileExtension = splited[0]
		if splited[0] == "application" {
			fileExtension = splited[1]
		}
	}

	switch fileExtension {
	case "go", "java", "rs", "rb", "c", "cgi", "class", "cpp", "cs", "h",
		"php", "py", "sh", "swift", "vb", "css":
		return []string{"SOURCE"}
	case "txt", "text", "pdf", "md", "doc", "docx", "epub",
		"ppt", "pptx", "pps", "odp", "xls", "xlsm", "xlsx":
		return []string{"TEXT", "DOCUMENTATION"}
	case "yml", "yaml", "json":
		return []string{"TEXT"}
	case "exe", "a", "o", "octet-stream", "apk", "bat",
		"bin", "pl", "com", "gadget", "jar", "msi", "wsf":
		return []string{"BINARY", "APPLICATION"}
	case "jpeg", "jpg", "png", "svg", "ai", "bmp", "gif", "ico",
		"ps", "psd", "tif", "tiff":
		return []string{"IMAGE"}
	case "mp3", "wav", "aif", "cda", "mid", "midi",
		"mpa", "ogg", "wma", "wpl":
		return []string{"AUDIO"}
	case "zip", "tar", "tar.gz", "tar.bz2", "7z", "arj",
		"deb", "pkg", "rar", "rpm", "z":
		return []string{"ARCHIVE"}
	default:
		return []string{"OTHER"}
	}
}

func getFileContentType(path string) (string, error) {
	// Only the first 512 bytes are used to sniff the content type.
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		return "", err
	}

	contentType := http.DetectContentType(buffer)
	return contentType, nil
}
