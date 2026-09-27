package combined

import (
	"context"
	"errors"

	"github.com/desain-gratis/common/lib/raft"
)

var _ raft.ApplicationV2 = &combineApp{}

var ErrUnsupportedCommand = errors.New("unsupported commmand")

type combineApp struct {
	apps []raft.ApplicationV2
}

func New(apps ...raft.ApplicationV2) *combineApp {
	return &combineApp{
		apps: apps,
	}
}

// make it easier for everyone..
func (m *combineApp) OnUpdateV2(ctx context.Context, e raft.EntryV2) (any, error) {
	var result any
	var err error
	for _, app := range m.apps {
		result, err = app.OnUpdateV2(ctx, e)
		if errors.Is(err, errors.ErrUnsupported) {
			// look for supporting app in the app sequence
			continue
		}
		break
	}

	return result, err
}
