package uimeta

import (
	"path/filepath"

	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/service/auth"
)

// Option is a backend-authored selectable value for the UI.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"`
}

// Capability describes whether a UI action should be shown and how it behaves.
type Capability struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode,omitempty"`
	Label   string `json:"label,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

type AppMeta struct {
	Name       string `json:"name"`
	Env        string `json:"env"`
	LogoLetter string `json:"logo_letter"`
}

type AuthMeta struct {
	LoginHint              string `json:"login_hint"`
	LoginEmailPlaceholder  string `json:"login_email_placeholder,omitempty"`
	RememberMe             bool   `json:"remember_me"`
	BasicAuth              bool   `json:"basic_auth_enabled"`
	Lockscreen             bool   `json:"lockscreen_enabled"`
	LockAfterSecs          int    `json:"lock_after_seconds"`
}

type NavItem struct {
	Path  string `json:"path"`
	Label string `json:"label"`
	Group string `json:"group"`
	Icon  string `json:"icon"`
}

type LogsMeta struct {
	TailKinds []Option `json:"tail_kinds"`
}

type FilesMeta struct {
	Roots   []auth.FileRoot `json:"roots"`
	Actions []Option        `json:"actions"`
}

type NginxMeta struct {
	Test   Capability `json:"test"`
	Reload Capability `json:"reload"`
}

type CronMeta struct {
	RunEveryOptions []Option   `json:"run_every_options"`
	ManualRun       Capability `json:"manual_run"`
}

type MountsMeta struct {
	DefaultOptions string   `json:"default_options"`
	DumpDefault    string   `json:"dump_default"`
	FsckDefault    string   `json:"fsck_default"`
	FSTypes        []Option `json:"fs_types"`
	Example        string   `json:"example"`
}

type TrafficMeta struct {
	Ranges []Option `json:"ranges"`
}

type DockerMeta struct {
	Restart Capability `json:"restart"`
	Stop    Capability `json:"stop"`
	Logs    Capability `json:"logs"`
}

type WebsiteMeta struct {
	Types             []Option `json:"types"`
	WebRoot           string   `json:"web_root"`
	StaticPathHint    string   `json:"static_path_hint"`
	ProxyUpstreamHint string   `json:"proxy_upstream_hint"`
}

// Response is returned by GET /ui/meta.
type Response struct {
	App        AppMeta     `json:"app"`
	Auth       AuthMeta    `json:"auth"`
	Navigation []NavItem   `json:"navigation"`
	Files      FilesMeta   `json:"files"`
	Logs       LogsMeta    `json:"logs"`
	Nginx      NginxMeta   `json:"nginx"`
	Cron       CronMeta    `json:"cron"`
	Mounts     MountsMeta  `json:"mounts"`
	Traffic    TrafficMeta `json:"traffic"`
	Docker     DockerMeta  `json:"docker"`
	Websites   WebsiteMeta `json:"websites"`
}

// Service builds backend-owned UI metadata from runtime config.
type Service struct {
	cfg config.Config
}

func NewService(cfg config.Config) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) Get() Response {
	fileRoots := auth.LoginMetadataFromConfig(
		s.cfg.EnableLockscreen,
		s.cfg.AuthEnable,
		int(s.cfg.LockAfter.Seconds()),
		s.cfg.WebPath,
		s.cfg.Storage,
	).FileRoots
	nginxMode := "real"
	nginxHint := "Runs nginx test/reload on the host."
	if s.cfg.AppEnv == "local" {
		nginxMode = "noop"
		nginxHint = "Local mode validates the UI flow without reloading host nginx."
	}
	webHint := filepath.Join(s.cfg.WebPath, "example-site")
	loginHint := "Sign in with your panel email and password"
	loginEmailPlaceholder := ""
	if s.cfg.AppEnv == "local" {
		loginHint = "Demo sign-in: admin@demo.com / 123456"
		loginEmailPlaceholder = "admin@demo.com"
	}
	return Response{
		App: AppMeta{Name: "GoSite", Env: s.cfg.AppEnv, LogoLetter: "G"},
		Auth: AuthMeta{
			LoginHint:             loginHint,
			LoginEmailPlaceholder: loginEmailPlaceholder,
			RememberMe:            true,
			BasicAuth:             s.cfg.AuthEnable,
			Lockscreen:            s.cfg.EnableLockscreen,
			LockAfterSecs:         int(s.cfg.LockAfter.Seconds()),
		},
		Navigation: []NavItem{
			{Path: "/dashboard", Label: "Dashboard", Group: "Observe", Icon: "dashboard"},
			{Path: "/websites", Label: "Websites", Group: "Operate", Icon: "globe"},
			{Path: "/metrics", Label: "Traffic", Group: "Observe", Icon: "chart"},
			{Path: "/logs", Label: "Logs", Group: "Observe", Icon: "search"},
			{Path: "/files", Label: "Files", Group: "Storage", Icon: "folder"},
			{Path: "/database", Label: "Database", Group: "Storage", Icon: "database"},
			{Path: "/docker", Label: "Docker", Group: "Runtime", Icon: "docker"},
			{Path: "/cron", Label: "Cron", Group: "Runtime", Icon: "clock"},
			{Path: "/mounts", Label: "Mounts", Group: "Runtime", Icon: "disk"},
			{Path: "/nginx", Label: "Nginx", Group: "Config", Icon: "server"},
			{Path: "/settings", Label: "Settings", Group: "Config", Icon: "settings"},
		},
		Logs: LogsMeta{
			TailKinds: []Option{
				{Value: "access", Label: "Access log"},
				{Value: "error", Label: "Error log"},
			},
		},
		Files: FilesMeta{
			Roots: fileRoots,
			Actions: []Option{
				{Value: "read", Label: "Read file"},
				{Value: "create_file", Label: "Create file"},
				{Value: "create_folder", Label: "Create folder"},
				{Value: "upload", Label: "Upload"},
				{Value: "delete", Label: "Delete"},
				{Value: "chmod", Label: "Change permissions"},
				{Value: "copy", Label: "Copy"},
			},
		},
		Nginx: NginxMeta{
			Test:   Capability{Enabled: true, Mode: nginxMode, Label: "Test config", Hint: nginxHint},
			Reload: Capability{Enabled: true, Mode: nginxMode, Label: "Reload nginx", Hint: nginxHint},
		},
		Cron: CronMeta{
			RunEveryOptions: []Option{
				{Value: "min", Label: "Every minute", Hint: "Runs when the minute changes."},
				{Value: "hour", Label: "Hourly", Hint: "Runs when the hour changes."},
				{Value: "day", Label: "Daily", Hint: "Runs when the day changes."},
				{Value: "month", Label: "Monthly", Hint: "Runs when the month changes."},
			},
			ManualRun: Capability{Enabled: true, Label: "Run now", Hint: "Runs the payload asynchronously and streams output."},
		},
		Mounts: MountsMeta{
			DefaultOptions: "defaults",
			DumpDefault:    "0",
			FsckDefault:    "0",
			FSTypes: []Option{
				{Value: "nfs", Label: "NFS"},
				{Value: "cifs", Label: "CIFS / SMB"},
				{Value: "ext4", Label: "ext4"},
				{Value: "xfs", Label: "XFS"},
				{Value: "none", Label: "Bind / none"},
			},
			Example: "Example: device nfs:/export, mount point /storage/mnt/site-assets, type nfs, options rw,nfsvers=4.",
		},
		Traffic: TrafficMeta{Ranges: []Option{
			{Value: "1h", Label: "Last hour"},
			{Value: "6h", Label: "Last 6 hours"},
			{Value: "24h", Label: "Last 24 hours"},
			{Value: "7d", Label: "Last 7 days"},
		}},
		Docker: DockerMeta{
			Restart: Capability{Enabled: true, Label: "Restart", Hint: "Restart a running container."},
			Stop:    Capability{Enabled: true, Label: "Stop", Hint: "Stop a running container after confirmation."},
			Logs:    Capability{Enabled: true, Label: "View logs"},
		},
		Websites: WebsiteMeta{
			Types: []Option{
				{Value: "static", Label: "Static site", Hint: "Serve files from a folder."},
				{Value: "proxy", Label: "Reverse proxy", Hint: "Forward traffic to an upstream service."},
			},
			WebRoot:           s.cfg.WebPath,
			StaticPathHint:    webHint,
			ProxyUpstreamHint: "http://127.0.0.1:3000",
		},
	}
}
