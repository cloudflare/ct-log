package main

import (
	"fmt"
	"go/build"
	"log"
	"os"

	"github.com/markbates/deplist"
)

func main() {
	pwd, _ := os.Getwd()
	deps, err := deplist.FindImports(pwd, build.IgnoreVendor)
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range deps {
		fmt.Println(d)
	}
}
