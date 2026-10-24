package preview

import (
	"context"
	"github.com/lsy223622/BloGo/internal/build"
	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/server"
	"io"
)

type Handler = server.Handler

func NewHandler(result *build.Result) (*Handler, error) { return server.NewPreviewHandler(result) }
func Dev(ctx context.Context, c *config.Config, output, listen string, log io.Writer) error {
	return server.Dev(ctx, c, output, listen, log)
}
