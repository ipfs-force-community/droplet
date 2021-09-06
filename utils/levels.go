package utils

import (
	"os"

	logging "github.com/ipfs/go-log/v2"
)

func SetupLogLevels() {
	if _, set := os.LookupEnv("GOLOG_LOG_LEVEL"); !set {
		_ = logging.SetLogLevel("*", "INFO")
	}
}
