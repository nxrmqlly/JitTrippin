package main

import (
	"net/http"

	"github.com/nxrmqlly/jittrippin/internal/server"
)

func main() {
	h, err := server.NewServer()
	if err != nil {
		panic(err)
	}

	srv := http.Server{
		Handler: h,
		Addr:    ":5500",
	}
	srv.ListenAndServe()
}
