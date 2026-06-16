package app

import (
	"context"
	"fmt"

	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/infra/nginx"
)

// RunNginxRepair validates nginx configuration and applies safe automatic fixes.
func RunNginxRepair(cfg config.Config) error {
	svc := nginx.NewServiceFromConfig(cfg, nil)

	actions, err := svc.TestAndRepair(context.Background())
	for _, action := range actions {
		fmt.Printf("gosite nginx-repair: %s:%d %s -> %s\n", action.File, action.Line, action.Reason, action.Fix)
	}
	if err != nil {
		return err
	}
	if len(actions) == 0 {
		fmt.Println("gosite nginx-repair: configuration ok")
	} else {
		fmt.Printf("gosite nginx-repair: applied %d fix(es), configuration ok\n", len(actions))
	}
	return nil
}
