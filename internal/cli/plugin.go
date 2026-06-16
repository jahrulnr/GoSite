package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	pluginsvc "github.com/jahrulnr/gosite/internal/service/plugin"
	"github.com/jahrulnr/gosite/internal/service/plugin/catalog"
	pluginremote "github.com/jahrulnr/gosite/internal/service/plugin/remote"
)

// RunPlugin handles `gosite plugin` subcommands (G6).
func RunPlugin(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gosite plugin <list|resolve|install|catalog>")
	}
	cfg := config.Load()
	db, err := sqlite.Open(cfg.Database)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := sqlite.Migrate(db, cfg.MigrationsDir); err != nil {
		return err
	}

	pluginRepo := sqlite.NewPluginRepository(db)
	pluginSvc := pluginsvc.NewService(
		pluginRepo,
		cfg.Storage,
		pluginsvc.NoopRuntimeManager{},
		pluginsvc.NoopHookDispatcher{},
		pluginsvc.WithAllowUnsigned(cfg.PluginAllowUnsigned),
		pluginsvc.WithKeyringPath(cfg.PluginKeyringPath),
		pluginsvc.WithHostVersion(cfg.AppVersion),
	)
	remoteCfg := pluginremote.ConfigFromApp(cfg)
	remoteSvc := pluginremote.NewService(remoteCfg)
	catalogSvc := catalog.NewService(cfg.PluginCatalogPath)
	ctx := context.Background()

	switch args[0] {
	case "list":
		plugins, err := pluginSvc.List(ctx)
		if err != nil {
			return err
		}
		for _, p := range plugins {
			fmt.Printf("%s@%s %s\n", p.PluginID, p.Version, p.State)
		}
		return nil
	case "catalog":
		fs := flag.NewFlagSet("plugin catalog", flag.ExitOnError)
		query := fs.String("q", "", "search query")
		_ = fs.Parse(args[1:])
		entries, err := catalogSvc.List(ctx, *query)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	case "resolve":
		source, err := parseSourceFlag(args[1:])
		if err != nil {
			return err
		}
		preview, err := remoteSvc.Resolve(ctx, source)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(preview)
	case "install":
		fs := flag.NewFlagSet("plugin install", flag.ExitOnError)
		sourceJSON := fs.String("source", "", "install source JSON")
		permissionsAck := fs.Bool("permissions-ack", false, "acknowledge permissions")
		_ = fs.Parse(args[1:])
		if strings.TrimSpace(*sourceJSON) == "" {
			return fmt.Errorf("--source JSON is required")
		}
		var source pluginremote.Source
		if err := json.Unmarshal([]byte(*sourceJSON), &source); err != nil {
			return fmt.Errorf("parse source: %w", err)
		}
		plan, data, err := remoteSvc.ResolveAndFetch(ctx, source, "")
		if err != nil {
			return err
		}
		record, err := pluginSvc.Install(ctx, pluginsvc.InstallInput{
			Name:           "plugin.zip",
			Content:        data,
			ExpectedSHA256: plan.SHA256,
			PermissionsAck: *permissionsAck,
			Provenance: &pluginsvc.InstallProvenance{
				SourceType:       plan.SourceType,
				SourceRef:        plan.SourceRef,
				ResolvedURL:      plan.URL,
				ResolvedDigest:   plan.ResolvedDigest,
				SourceCommit:     plan.SourceCommit,
				SourceRepository: plan.SourceRepository,
				InstallPath:      plan.InstallPath,
			},
		})
		if err != nil {
			return err
		}
		fmt.Printf("installed %s@%s state=%s\n", record.PluginID, record.Version, record.State)
		return nil
	default:
		return fmt.Errorf("unknown plugin command: %s", args[0])
	}
}

func parseSourceFlag(args []string) (pluginremote.Source, error) {
	fs := flag.NewFlagSet("plugin resolve", flag.ExitOnError)
	sourceJSON := fs.String("source", "", "install source JSON")
	_ = fs.Parse(args)
	if strings.TrimSpace(*sourceJSON) == "" {
		return pluginremote.Source{}, fmt.Errorf("--source JSON is required")
	}
	var source pluginremote.Source
	if err := json.Unmarshal([]byte(*sourceJSON), &source); err != nil {
		return pluginremote.Source{}, fmt.Errorf("parse source: %w", err)
	}
	return source, nil
}
