package main

import (
	"log"

	"github.com/nxrmqlly/jittrippin/internal/server"
)

func main() {
	srv := server.NewServer()

	if err := srv.Run(":5500"); err != nil {
		log.Fatal(err)
	}
}
