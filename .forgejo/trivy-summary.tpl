{{- $severities := list "CRITICAL" "HIGH" "MEDIUM" "LOW" "UNKNOWN" -}}
{{- $icons := dict "CRITICAL" "🟣" "HIGH" "🔴" "MEDIUM" "🟠" "LOW" "🟡" "UNKNOWN" "⚪" -}}
{{- $found := false -}}
{{- range . }}{{ if .Vulnerabilities }}{{ $found = true }}{{ end }}{{ end -}}
{{- if not $found }}
No vulnerabilities found.
{{ else }}
| Target | 🟣 Critical | 🔴 High | 🟠 Medium | 🟡 Low | ⚪ Unknown |
| --- | ---: | ---: | ---: | ---: | ---: |
{{- range . }}
{{- if .Vulnerabilities }}
{{- $counts := dict "CRITICAL" 0 "HIGH" 0 "MEDIUM" 0 "LOW" 0 "UNKNOWN" 0 }}
{{- range .Vulnerabilities }}{{ $_ := set $counts .Severity (add1 (get $counts .Severity)) }}{{ end }}
| `{{ .Target }}` | {{ get $counts "CRITICAL" }} | {{ get $counts "HIGH" }} | {{ get $counts "MEDIUM" }} | {{ get $counts "LOW" }} | {{ get $counts "UNKNOWN" }} |
{{- end }}
{{- end }}

{{ range . }}
{{- if .Vulnerabilities }}
{{- $vulns := .Vulnerabilities }}
<details>
<summary><code>{{ .Target }}</code> ({{ .Type }}): {{ len .Vulnerabilities }}</summary>

| | ID | Package | Installed | Fixed |
| --- | --- | --- | --- | --- |
{{- range $sev := $severities }}
{{- range $vulns }}
{{- if eq .Severity $sev }}
| {{ get $icons .Severity }} | {{ if .PrimaryURL }}[{{ .VulnerabilityID }}]({{ .PrimaryURL }} "{{ .Title | replace "\"" "'" | replace "\n" " " }}"){{ else }}{{ .VulnerabilityID }}{{ end }} | {{ .PkgName }} | {{ .InstalledVersion }} | {{ .FixedVersion | replace ", " "<br>" }} |
{{- end }}
{{- end }}
{{- end }}

</details>
{{ end }}
{{- end }}
{{- end }}
