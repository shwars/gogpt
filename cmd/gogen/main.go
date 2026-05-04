package main

import (
	"os"

	"github.com/shwars/gogpt/internal/gogen"
)

func main() {
	os.Exit(gogen.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
