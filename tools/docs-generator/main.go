package main

import (
	"os"

	docs "github.com/urfave/cli-docs/v3"

	"altlinux.space/alt-atomic-lab/selenite/internal/serve"
)

func main() {
	sshApp := serve.AppCmd(nil)

	md, err := docs.ToMarkdown(sshApp)
	if err != nil {
		panic(err)
	}

	fi, err := os.Create("SSH-DOCS.md")
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = fi.Close()
	}()
	if _, err := fi.WriteString("# CLI\n\n" + md); err != nil {
		panic(err)
	}
}
