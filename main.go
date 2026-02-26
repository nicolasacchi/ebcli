package main

import (
	"os"

	"github.com/nicolasacchi/ebcli/cmd"
)

func main() {
	if err := cmd.Execute(Version); err != nil {
		os.Exit(1)
	}
}
