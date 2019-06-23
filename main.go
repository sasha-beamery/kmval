package main

import (
	"fmt"
	"os"

	"github.com/LGUG2Z/kmval/cli"
)

func main() {
	if err := cli.App().Run(os.Args); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
