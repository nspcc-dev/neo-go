package main

import (
	"os"

	log "github.com/go-kit/kit/log"
)

func main() {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger.Log("hello", true)

	logger = log.With(logger, "module", "node")
	logger.Log("foo", true)
}
