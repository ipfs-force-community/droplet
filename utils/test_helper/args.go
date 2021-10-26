package test_helper

import (
	"flag"
	"testing"
)

// use "go test ..." with suffix '-local=true' to set 'local' args
var localTest = flag.Bool("local", false, "Run local go tests")
var mysqlConn = flag.String("mysql", "", "connection string of testing mysql database")

func Mysql(t *testing.T) string {
	if len(*mysqlConn) == 0 {
		t.Skipf("skip %s, without -mysql args", t.Name())
	}
	return *mysqlConn
}

func LocalTest(t *testing.T) {
	if !*localTest {
		t.Skipf("skip %s, not a local test", t.Name())
	}
}
