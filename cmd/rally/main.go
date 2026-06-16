package main

import (
	"log"
	"os"

	"github.com/ZMenggg/Rally/internal/cli"
)

func main() {
	app := cli.NewApp()
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
