package main

import (
	"os"

	"github.com/shwars/gogpt/internal/gogpt"
)

func main() {
	os.Exit(gogpt.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
