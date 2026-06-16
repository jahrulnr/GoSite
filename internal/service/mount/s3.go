package mount

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jahrulnr/gosite/pkg/apperror"
)

const s3FSType = "fuse.s3fs"

var s3BucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

// S3Config holds S3 mount settings. Access/secret keys are write-only in API responses.
type S3Config struct {
	Bucket    string `json:"bucket,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
	Region    string `json:"region,omitempty"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
	PathStyle bool   `json:"path_style,omitempty"`
}

func IsS3Type(typ string) bool {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case s3FSType, "s3", "s3fs":
		return true
	default:
		return false
	}
}

func normalizeS3Type(typ string) string {
	if IsS3Type(typ) {
		return s3FSType
	}
	return typ
}

func mountSecretName(dir string) string {
	name := strings.Trim(strings.TrimSpace(dir), "/")
	name = strings.ReplaceAll(name, "/", "-")
	if name == "" {
		name = "root"
	}
	return "s3-" + name + ".passwd"
}

func passwdPath(secretsDir, dir string) string {
	return filepath.Join(secretsDir, mountSecretName(dir))
}

func writePasswdFile(path, accessKey, secretKey string) error {
	if strings.TrimSpace(accessKey) == "" || strings.TrimSpace(secretKey) == "" {
		return apperror.New(apperror.CodeInvalidInput, "S3 access key and secret key required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "create mount secrets dir", err)
	}
	content := strings.TrimSpace(accessKey) + ":" + strings.TrimSpace(secretKey) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "write S3 credentials", err)
	}
	return nil
}

func validateS3Bucket(bucket string) error {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return apperror.New(apperror.CodeInvalidInput, "S3 bucket required")
	}
	if len(bucket) < 3 || len(bucket) > 63 {
		return apperror.New(apperror.CodeInvalidInput, "S3 bucket name must be 3-63 characters")
	}
	if !s3BucketPattern.MatchString(strings.ToLower(bucket)) {
		return apperror.New(apperror.CodeInvalidInput, "invalid S3 bucket name")
	}
	return nil
}

func normalizeS3Endpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", nil
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return "", apperror.New(apperror.CodeInvalidInput, "invalid S3 endpoint URL")
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func buildS3Options(cfg S3Config, passwdFile string) (string, error) {
	endpoint, err := normalizeS3Endpoint(cfg.Endpoint)
	if err != nil {
		return "", err
	}
	opts := []string{
		"_netdev",
		"allow_other",
		"passwd_file=" + passwdFile,
	}
	if endpoint != "" {
		opts = append(opts, "url="+endpoint)
	}
	if strings.TrimSpace(cfg.Region) != "" {
		opts = append(opts, "endpoint="+strings.TrimSpace(cfg.Region))
	}
	if cfg.PathStyle {
		opts = append(opts, "use_path_request_style")
	}
	return strings.Join(opts, ","), nil
}

func parsePasswdFileOption(options string) string {
	for _, part := range strings.Split(options, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "passwd_file=") {
			return strings.TrimPrefix(part, "passwd_file=")
		}
	}
	return ""
}

func parseS3Options(options string) S3Config {
	cfg := S3Config{}
	for _, part := range strings.Split(options, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "url="):
			cfg.Endpoint = strings.TrimPrefix(part, "url=")
		case strings.HasPrefix(part, "endpoint="):
			cfg.Region = strings.TrimPrefix(part, "endpoint=")
		case part == "use_path_request_style":
			cfg.PathStyle = true
		}
	}
	return cfg
}

func enrichS3ForList(entry Entry) Entry {
	if !IsS3Type(entry.Type) {
		return entry
	}
	cfg := parseS3Options(entry.Options)
	cfg.Bucket = entry.Device
	entry.S3 = &cfg
	return entry
}

func (s *Service) applyS3Entry(entry Entry, keepPasswdPath string, requireKeys bool) (Entry, error) {
	if entry.S3 == nil {
		return Entry{}, apperror.New(apperror.CodeInvalidInput, "S3 settings required")
	}
	cfg := *entry.S3
	if err := validateS3Bucket(cfg.Bucket); err != nil {
		return errEntry(err)
	}

	passwd := keepPasswdPath
	if passwd == "" {
		passwd = passwdPath(s.secretsDir, entry.Dir)
	}
	hasKeys := strings.TrimSpace(cfg.AccessKey) != "" && strings.TrimSpace(cfg.SecretKey) != ""
	if requireKeys && !hasKeys {
		return Entry{}, apperror.New(apperror.CodeInvalidInput, "S3 access key and secret key required")
	}
	if hasKeys {
		if err := writePasswdFile(passwd, cfg.AccessKey, cfg.SecretKey); err != nil {
			return Entry{}, err
		}
	} else if requireKeys || keepPasswdPath == "" {
		if _, err := os.Stat(passwd); err != nil {
			return Entry{}, apperror.New(apperror.CodeInvalidInput, "S3 credentials missing; provide access key and secret key")
		}
	}

	options, err := buildS3Options(cfg, passwd)
	if err != nil {
		return Entry{}, err
	}

	entry.Device = strings.TrimSpace(cfg.Bucket)
	entry.Type = s3FSType
	entry.Options = options
	entry.Dump = "0"
	entry.Fsck = "0"
	entry.S3 = nil
	return entry, nil
}

func errEntry(err error) (Entry, error) {
	return Entry{}, err
}

func removePasswdFromOptions(options string) {
	path := parsePasswdFileOption(options)
	if path != "" {
		_ = os.Remove(path)
	}
}
