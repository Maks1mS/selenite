// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"text/template"

	"altlinux.space/alt-atomic/selenite/internal/cmdutils"
)

const alloyConfigPath = "/etc/alloy/config.alloy"

var alloyConfigTmpl = template.Must(template.New("alloy").Parse(`
{{- if .Prometheus.Enabled }}
prometheus.exporter.unix "host" {
  include_exporter_metrics = true
}

prometheus.scrape "host_metrics" {
  targets    = prometheus.exporter.unix.host.targets
  forward_to = [prometheus.remote_write.metrics.receiver]
}

prometheus.remote_write "metrics" {
  endpoint {
    url = {{ printf "%q" .Prometheus.Endpoint }}
{{ if .Prometheus.Auth.Username }}
    basic_auth {
      username = {{ printf "%q" .Prometheus.Auth.Username }}
      password = {{ printf "%q" .Prometheus.Auth.Password }}
    }
{{ end -}}
  }
}
{{ end -}}
{{- if .Loki.Enabled }}
loki.relabel "journal" {
  forward_to = []

  rule {
    source_labels = ["__journal__systemd_unit"]
    target_label  = "unit"
  }

  rule {
    source_labels = ["__journal_container_name"]
    target_label  = "container"
  }

  rule {
    source_labels = ["__journal_image_name"]
    target_label  = "image"
  }

  rule {
    source_labels = ["container"]
    regex         = "^(.+)_(.+)_\\d+$"
    replacement   = "$1"
    target_label  = "compose_project"
  }
  rule {
    source_labels = ["container"]
    regex         = "^(.+)_(.+)_\\d+$"
    replacement   = "$2"
    target_label  = "compose_service"
  }

  rule {
    source_labels = ["container"]
    regex         = "(.+)"
    replacement   = "podman"
    target_label  = "job"
  }
}

loki.source.journal "journal" {
  forward_to    = [loki.write.logs.receiver]
  relabel_rules = loki.relabel.journal.rules
  labels        = { job = "system" }
  max_age       = "12h"
}

loki.write "logs" {
  endpoint {
    url = {{ printf "%q" .Loki.Endpoint }}
{{ if .Loki.Auth.Username }}
    basic_auth {
      username = {{ printf "%q" .Loki.Auth.Username }}
      password = {{ printf "%q" .Loki.Auth.Password }}
    }
{{ end -}}
  }
  external_labels = {
    host = constants.hostname,
  }
}
{{ end -}}
`))

type authParams struct {
	Username string
	Password string
}

type endpointParams struct {
	Enabled  bool
	Endpoint string
	Auth     authParams
}

type alloyParams struct {
	Prometheus endpointParams
	Loki       endpointParams
}

func main() {
	runtime.LockOSThread()
	if err := syscall.Setresgid(0, 0, 0); err != nil {
		cmdutils.Fatal("setresgid: %v\n", err)
	}
	if err := syscall.Setresuid(0, 0, 0); err != nil {
		cmdutils.Fatal("setresuid: %v\n", err)
	}

	mode := "apply"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	switch mode {
	case "apply":
		runApply()
	default:
		cmdutils.Fatal("unknown mode: %s\n", mode)
	}
}

func runApply() {
	if os.Getenv("SELENITE_ALLOY_HELPER_ENABLED") != "1" {
		mustRun("/usr/bin/systemctl", "disable", "--now", "alloy")
		return
	}

	params := alloyParams{
		Prometheus: endpointParams{
			Enabled:  os.Getenv("SELENITE_ALLOY_HELPER_PROMETHEUS_ENABLED") == "1",
			Endpoint: os.Getenv("SELENITE_ALLOY_HELPER_PROMETHEUS_ENDPOINT"),
			Auth: authParams{
				Username: os.Getenv("SELENITE_ALLOY_HELPER_PROMETHEUS_USERNAME"),
				Password: os.Getenv("SELENITE_ALLOY_HELPER_PROMETHEUS_PASSWORD"),
			},
		},
		Loki: endpointParams{
			Enabled:  os.Getenv("SELENITE_ALLOY_HELPER_LOKI_ENABLED") == "1",
			Endpoint: os.Getenv("SELENITE_ALLOY_HELPER_LOKI_ENDPOINT"),
			Auth: authParams{
				Username: os.Getenv("SELENITE_ALLOY_HELPER_LOKI_USERNAME"),
				Password: os.Getenv("SELENITE_ALLOY_HELPER_LOKI_PASSWORD"),
			},
		},
	}

	var buf bytes.Buffer
	if err := alloyConfigTmpl.Execute(&buf, params); err != nil {
		cmdutils.Fatal("render alloy config: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(alloyConfigPath), 0o755); err != nil { //gosec:disable G301 -- Expected
		cmdutils.Fatal("mkdir alloy config dir: %v", err)
	}
	if err := os.WriteFile(alloyConfigPath, buf.Bytes(), 0o644); err != nil { //gosec:disable G306 -- Expected
		cmdutils.Fatal("write alloy config: %v", err)
	}

	mustRun("/usr/bin/systemctl", "enable", "--now", "alloy")
}

func mustRun(name string, args ...string) {
	//gosec:disable G204 -- Expected
	cmd := exec.Command(name, args...)
	if err := cmd.Run(); err != nil {
		cmdutils.Fatal("%s %v: %v", name, args, err)
	}
}
