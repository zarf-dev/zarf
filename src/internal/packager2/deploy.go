package packager2

import (
	"context"

	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

func Deploy(ctx context.Context, pkgLayout *layout.PackageLayout) error {
	l := logger.From(ctx)
	l.Info("starting deploy")

	return nil
}
