package utils

import (
	"os"

	logging "github.com/ipfs/go-log/v2"
)

func SetupLogLevels() {
	if val, set := os.LookupEnv("GOLOG_LOG_LEVEL"); !set {
		_ = logging.SetLogLevel("*", "DEBUG")
		_ = logging.SetLogLevel("addrutil", "INFO")
		_ = logging.SetLogLevel("rpc", "INFO")
		_ = logging.SetLogLevel("badger", "INFO")
		_ = logging.SetLogLevel("basichost", "INFO")
		_ = logging.SetLogLevel("events", "INFO")
		_ = logging.SetLogLevel("fsm", "INFO")
		_ = logging.SetLogLevel("evtsm", "INFO")
		_ = logging.SetLogLevel("dagstore/upgrader", "INFO")
	} else {
		_ = logging.SetLogLevel("*", val)
	}
}
