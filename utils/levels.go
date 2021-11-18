package utils

import (
	"os"
)

func SetupLogLevels() {
	if _, set := os.LookupEnv("GOLOG_LOG_LEVEL"); !set {
		/*	_ = logging.SetLogLevel("*", "DEBUG")
			_ = logging.SetLogLevel("addrutil", "INFO")
			_ = logging.SetLogLevel("rpc", "INFO")
			_ = logging.SetLogLevel("badger", "INFO")
			_ = logging.SetLogLevel("basichost", "INFO")
			_ = logging.SetLogLevel("events", "INFO")
			_ = logging.SetLogLevel("fsm", "INFO")
			_ = logging.SetLogLevel("evtsm", "INFO")
			_ = logging.SetLogLevel("dagstore/upgrader", "INFO")*/
	}
}
