package journal

import (
	"context"
	"github.com/filecoin-project/venus-market/config"
	"go.uber.org/fx"
)

func OpenFilesystemJournal(homeDir *config.HomeDir, lc fx.Lifecycle, disabled DisabledEvents) (Journal, error) {
	jrnl, err := OpenFSJournal(string(*homeDir), disabled)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error { return jrnl.Close() },
	})

	return jrnl, err
}
