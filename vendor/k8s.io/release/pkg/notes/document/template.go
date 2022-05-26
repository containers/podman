/*
Copyright 2020 The Kubernetes Authors.

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

package document

const ContainerImagesDescription = `### Container Images

All container images are available as manifest lists and support the described
architectures. It is also possible to pull a specific architecture directly by
adding the "-$ARCH" suffix  to the container image name.
`

// defaultReleaseNotesTemplate is the text template for the default release notes.
// k8s/release/cmd/release-notes uses text/template to render markdown
// templates.
const defaultReleaseNotesTemplate = `
{{- $CurrentRevision := .CurrentRevision -}}
{{- $PreviousRevision := .PreviousRevision -}}

{{if .FileDownloads}}
## Downloads for {{$CurrentRevision}}

{{- with .FileDownloads.Source }}

### Source Code

filename | sha512 hash
-------- | -----------
{{range .}}[{{.Name}}]({{.URL}}) | {{.Checksum}}{{println}}{{end}}
{{end}}

{{- with .FileDownloads.Client -}}
### Client Binaries

filename | sha512 hash
-------- | -----------
{{range .}}[{{.Name}}]({{.URL}}) | {{.Checksum}}{{println}}{{end}}
{{end}}

{{- with .FileDownloads.Server -}}
### Server Binaries

filename | sha512 hash
-------- | -----------
{{range .}}[{{.Name}}]({{.URL}}) | {{.Checksum}}{{println}}{{end}}
{{end}}

{{- with .FileDownloads.Node -}}
### Node Binaries

filename | sha512 hash
-------- | -----------
{{range .}}[{{.Name}}]({{.URL}}) | {{.Checksum}}{{println}}{{end}}
{{end -}}
{{- end -}}

{{with .CVEList -}}
## Important Security Information

This release contains changes that address the following vulnerabilities:
{{range .}}
### {{.ID}}: {{.Title}}

{{.Description}}

**CVSS Rating:** {{.CVSSRating}} ({{.CVSSScore}}) [{{.CVSSVector}}]({{.CalcLink}})
{{- if .TrackingIssue -}}
<br>
**Tracking Issue:** {{.TrackingIssue}}
{{- end }}

{{ end }}
{{- end -}}

{{with .NotesWithActionRequired -}}
## Urgent Upgrade Notes 

### (No, really, you MUST read this before you upgrade)

{{range .}}{{println "-" .}} {{end}}
{{end}}

{{- if .Notes -}}
## Changes by Kind
{{ range .Notes}}
### {{.Kind | prettyKind}}

{{range $note := .NoteEntries }}{{println "-" $note}}{{end}}
{{- end -}}
{{- end -}}
`
