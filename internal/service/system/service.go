package system

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// FS reads host files (typically /proc) for tests and production.
type FS interface {
	ReadFile(path string) ([]byte, error)
	Glob(pattern string) ([]string, error)
}

// OSFS reads from the real filesystem.
type OSFS struct{}

func (OSFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (OSFS) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

// CommandRunner runs shell utilities such as df.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (stdout string, err error)
}

// Service collects host metrics from /proc and df.
type Service struct {
	fs      FS
	cmd     CommandRunner
	logDir  string
	dfArgs  []string
}

// NewService returns a system metrics service.
func NewService(logDir string, fs FS, cmd CommandRunner) *Service {
	if fs == nil {
		fs = OSFS{}
	}
	return &Service{
		fs:     fs,
		cmd:    cmd,
		logDir: logDir,
		dfArgs: []string{"-P"},
	}
}

// MemoryStat is one row of memory usage (values in kilobytes).
type MemoryStat struct {
	Label string `json:"label"`
	Total int64  `json:"total"`
	Used  int64  `json:"used"`
	Free  int64  `json:"free"`
}

// StorageStat is overlay/root filesystem usage from df (kilobytes).
type StorageStat struct {
	System    string `json:"system"`
	Size      int64  `json:"size"`
	Used      int64  `json:"used"`
	Available int64  `json:"available"`
}

// InfoResult is CPU load percent, memory rows, and primary storage.
type InfoResult struct {
	CPU     float64      `json:"cpu"`
	Memory  []MemoryStat `json:"memory"`
	Storage *StorageStat `json:"storage"`
}

// NetworkResult holds cumulative RX/TX bytes per interface.
type NetworkResult struct {
	In  map[string]int64 `json:"in"`
	Out map[string]int64 `json:"out"`
}

// DiskIOResult summarizes disk activity from /proc/diskstats.
type DiskIOResult struct {
	Read  string `json:"read"`
	Write string `json:"write"`
}

// NginxTrafficResult aggregates nginx access log traffic.
type NginxTrafficResult struct {
	Sites map[string]contracts.TrafficSite `json:"sites"`
	Total contracts.TrafficSite            `json:"total"`
}

// Info returns CPU load, memory, and overlay storage usage.
func (s *Service) Info(ctx context.Context) (InfoResult, error) {
	cpu, err := s.cpuLoad()
	if err != nil {
		return InfoResult{}, apperror.Wrap(apperror.CodeInternal, "read cpu metrics", err)
	}

	memory, err := s.memoryFromProc()
	if err != nil {
		return InfoResult{}, apperror.Wrap(apperror.CodeInternal, "read memory metrics", err)
	}

	storage, err := s.storageFromDF(ctx)
	if err != nil {
		return InfoResult{}, apperror.Wrap(apperror.CodeInternal, "read storage metrics", err)
	}

	return InfoResult{
		CPU:     cpu,
		Memory:  memory,
		Storage: storage,
	}, nil
}

// Network returns RX/TX byte counters from /proc/net/dev.
func (s *Service) Network(_ context.Context) (NetworkResult, error) {
	data, err := s.fs.ReadFile("/proc/net/dev")
	if err != nil {
		return NetworkResult{}, apperror.Wrap(apperror.CodeInternal, "read network stats", err)
	}
	return parseNetDev(data), nil
}

// DiskIO returns aggregate read/write totals from /proc/diskstats.
func (s *Service) DiskIO(_ context.Context) (DiskIOResult, error) {
	data, err := s.fs.ReadFile("/proc/diskstats")
	if err != nil {
		return DiskIOResult{}, apperror.Wrap(apperror.CodeInternal, "read disk stats", err)
	}
	readSectors, writeSectors := parseDiskstats(data)
	return DiskIOResult{
		Read:  fmt.Sprintf("%d sectors", readSectors),
		Write: fmt.Sprintf("%d sectors", writeSectors),
	}, nil
}

// NginxTraffic parses access logs under logDir.
func (s *Service) NginxTraffic(_ context.Context) (NginxTrafficResult, error) {
	pattern := filepath.Join(s.logDir, "access*.log")
	files, err := s.fs.Glob(pattern)
	if err != nil {
		return NginxTrafficResult{}, apperror.Wrap(apperror.CodeInternal, "list access logs", err)
	}
	if len(files) == 0 {
		return NginxTrafficResult{
			Sites: map[string]contracts.TrafficSite{},
			Total: contracts.TrafficSite{},
		}, nil
	}

	result := NginxTrafficResult{
		Sites: make(map[string]contracts.TrafficSite),
	}
	for _, path := range files {
		base := filepath.Base(path)
		domain := "default"
		if strings.HasPrefix(base, "access-") {
			domain = strings.TrimSuffix(strings.TrimPrefix(base, "access-"), ".log")
		}
		stat, err := parseAccessLogFile(func() ([]byte, error) {
			return s.fs.ReadFile(path)
		})
		if err != nil {
			continue
		}
		result.Sites[domain] = stat
		result.Total.Requests += stat.Requests
		result.Total.Bytes += stat.Bytes
	}
	return result, nil
}

func (s *Service) cpuLoad() (float64, error) {
	loadData, err := s.fs.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(loadData))
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty loadavg")
	}
	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	cores, err := s.cpuCount()
	if err != nil || cores <= 0 {
		cores = 1
	}
	return round2(load1 / float64(cores) * 100), nil
}

func (s *Service) cpuCount() (int, error) {
	data, err := s.fs.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0, err
	}
	count := 0
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "processor") {
			count++
		}
	}
	if count == 0 {
		return 1, nil
	}
	return count, scanner.Err()
}

func (s *Service) memoryFromProc() ([]MemoryStat, error) {
	data, err := s.fs.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	values := parseMeminfo(data)
	memTotal := values["MemTotal"]
	memAvailable := values["MemAvailable"]
	if memAvailable == 0 {
		memAvailable = values["MemFree"]
	}
	memUsed := memTotal - memAvailable
	if memUsed < 0 {
		memUsed = 0
	}

	out := []MemoryStat{{
		Label: "mem",
		Total: memTotal,
		Used:  memUsed,
		Free:  memAvailable,
	}}

	if swapTotal := values["SwapTotal"]; swapTotal > 0 {
		swapFree := values["SwapFree"]
		swapUsed := swapTotal - swapFree
		if swapUsed < 0 {
			swapUsed = 0
		}
		out = append(out, MemoryStat{
			Label: "swap",
			Total: swapTotal,
			Used:  swapUsed,
			Free:  swapFree,
		})
	}
	return out, nil
}

func (s *Service) storageFromDF(ctx context.Context) (*StorageStat, error) {
	if s.cmd == nil {
		return nil, nil
	}
	stdout, err := s.cmd.Run(ctx, "df", s.dfArgs...)
	if err != nil {
		return nil, err
	}
	return parseDF(stdout), nil
}

func parseMeminfo(data []byte) map[string]int64 {
	out := make(map[string]int64)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		val, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		out[key] = val
	}
	return out
}

func parseDF(output string) *StorageStat {
	var fallback *StorageStat
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Filesystem") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		size, err1 := strconv.ParseInt(fields[1], 10, 64)
		used, err2 := strconv.ParseInt(fields[2], 10, 64)
		avail, err3 := strconv.ParseInt(fields[3], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		stat := &StorageStat{
			System:    fields[0],
			Size:      size,
			Used:      used,
			Available: avail,
		}
		if strings.Contains(line, "overlay") {
			return stat
		}
		if fallback == nil {
			fallback = stat
		}
	}
	return fallback
}

func parseNetDev(data []byte) NetworkResult {
	result := NetworkResult{
		In:  make(map[string]int64),
		Out: make(map[string]int64),
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		iface := strings.TrimSpace(parts[0])
		if iface == "lo" || iface == "" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}
		rx, _ := strconv.ParseInt(fields[0], 10, 64)
		tx, _ := strconv.ParseInt(fields[8], 10, 64)
		result.In[iface] = rx
		result.Out[iface] = tx
	}
	return result
}

func parseDiskstats(data []byte) (readSectors, writeSectors int64) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		read, _ := strconv.ParseInt(fields[5], 10, 64)
		write, _ := strconv.ParseInt(fields[9], 10, 64)
		readSectors += read
		writeSectors += write
	}
	return readSectors, writeSectors
}

var accessBytesPattern = regexp.MustCompile(`"[^"]*"\s+\d{3}\s+(\d+|-)\s+"`)

func parseAccessLogFile(read func() ([]byte, error)) (contracts.TrafficSite, error) {
	data, err := read()
	if err != nil {
		return contracts.TrafficSite{}, err
	}
	var stat contracts.TrafficSite
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		stat.Requests++
		if m := accessBytesPattern.FindStringSubmatch(line); len(m) == 2 {
			if m[1] != "-" {
				if b, err := strconv.ParseInt(m[1], 10, 64); err == nil {
					stat.Bytes += b
				}
			}
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			if b, err := strconv.ParseInt(fields[len(fields)-1], 10, 64); err == nil {
				stat.Bytes += b
			}
		}
	}
	return stat, scanner.Err()
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// MapFS is an in-memory FS for tests.
type MapFS struct {
	Files map[string][]byte
}

func (m MapFS) ReadFile(path string) ([]byte, error) {
	if data, ok := m.Files[path]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("file not found: %s", path)
}

func (m MapFS) Glob(pattern string) ([]string, error) {
	dir := filepath.Dir(pattern)
	basePattern := filepath.Base(pattern)
	var matches []string
	for path := range m.Files {
		if dir != "." && !strings.HasPrefix(path, dir) {
			continue
		}
		name := filepath.Base(path)
		ok, _ := filepath.Match(basePattern, name)
		if ok {
			matches = append(matches, path)
		}
	}
	return matches, nil
}

// ReadTailLines returns the last n lines from r.
func ReadTailLines(r io.Reader, n int) (string, error) {
	if n <= 0 {
		n = 100
	}
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[len(lines)-n:]
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}
