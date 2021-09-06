package builder

import (
	logging "github.com/ipfs/go-log/v2"

	"go.uber.org/fx"
)

var l = logging.Logger("fx")

type debugPrinter struct {
}

func (p *debugPrinter) Printf(f string, a ...interface{}) {
	l.Infof(f, a...)
}

var _ fx.Printer = new(debugPrinter)
