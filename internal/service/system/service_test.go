package system_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/service/system"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleProcFS() system.MapFS {
	return system.MapFS{Files: map[string][]byte{
		"/proc/loadavg": []byte("1.00 0.50 0.25 2/100 999\n"),
		"/proc/cpuinfo": []byte("processor\t: 0\nprocessor\t: 1\n"),
		"/proc/meminfo": []byte(`MemTotal:       8192000 kB
MemAvailable:   4096000 kB
SwapTotal:       1024000 kB
SwapFree:         512000 kB
`),
		"/proc/net/dev": []byte(`Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1000       10    0    0    0     0          0         0     2000       20    0    0    0     0       0          0
  eth0: 5000       50    0    0    0     0          0         0     7000       70    0    0    0     0       0          0
`),
		"/proc/diskstats": []byte(`   8       0 sda 100 0 5000 100 200 0 9000 200 0 0 0 0 0 0
`),
	}}
}

type stubCmd struct {
	stdout string
}

func (s stubCmd) Run(_ context.Context, _ string, _ ...string) (string, error) {
	return s.stdout, nil
}

func TestSystem_Info_CPUAndMemory(t *testing.T) {
	t.Parallel()
	svc := system.NewService(t.TempDir(), sampleProcFS(), stubCmd{
		stdout: "Filesystem     1024-blocks      Used Available Use% Mounted on\noverlay        1000000 500000 400000 56% /\n",
	})

	info, err := svc.Info(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 50.0, info.CPU)
	require.Len(t, info.Memory, 2)
	assert.Equal(t, int64(8192000), info.Memory[0].Total)
	assert.NotNil(t, info.Storage)
	assert.Equal(t, int64(1000000), info.Storage.Size)
}

func TestSystem_Info_NoDFCommand(t *testing.T) {
	t.Parallel()
	svc := system.NewService(t.TempDir(), sampleProcFS(), nil)
	info, err := svc.Info(context.Background())
	require.NoError(t, err)
	assert.Nil(t, info.Storage)
}

func TestSystem_Network_SkipsLoopback(t *testing.T) {
	t.Parallel()
	svc := system.NewService(t.TempDir(), sampleProcFS(), nil)
	net, err := svc.Network(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(5000), net.In["eth0"])
	assert.Equal(t, int64(7000), net.Out["eth0"])
	_, hasLo := net.In["lo"]
	assert.False(t, hasLo)
}

func TestSystem_DiskIO_AggregatesSectors(t *testing.T) {
	t.Parallel()
	svc := system.NewService(t.TempDir(), sampleProcFS(), nil)
	disk, err := svc.DiskIO(context.Background())
	require.NoError(t, err)
	assert.Contains(t, disk.Read, "5000 sectors")
	assert.Contains(t, disk.Write, "9000 sectors")
}

func TestSystem_NginxTraffic_DefaultLog(t *testing.T) {
	t.Parallel()
	logDir := t.TempDir()
	fs := sampleProcFS()
	fs.Files[logDir+"/access.log"] = []byte(strings.Join(testutil.SampleAccessLogLines, "\n") + "\n")
	svc := system.NewService(logDir, fs, nil)

	traffic, err := svc.NginxTraffic(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), traffic.Sites["default"].Requests)
	assert.Equal(t, int64(1323), traffic.Sites["default"].Bytes)
	assert.Equal(t, int64(2), traffic.Total.Requests)
}

func TestSystem_NginxTraffic_PerDomainLog(t *testing.T) {
	t.Parallel()
	logDir := t.TempDir()
	fs := sampleProcFS()
	fs.Files[logDir+"/access-example.com.log"] = []byte(testutil.SampleAccessLogLines[0] + "\n")
	svc := system.NewService(logDir, fs, nil)

	traffic, err := svc.NginxTraffic(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), traffic.Sites["example.com"].Requests)
}

func TestSystem_NginxTraffic_EmptyWhenNoLogs(t *testing.T) {
	t.Parallel()
	svc := system.NewService(t.TempDir(), system.MapFS{Files: map[string][]byte{}}, nil)
	traffic, err := svc.NginxTraffic(context.Background())
	require.NoError(t, err)
	assert.Empty(t, traffic.Sites)
}

func TestReadTailLines_ReturnsLastN(t *testing.T) {
	t.Parallel()
	content, err := system.ReadTailLines(strings.NewReader("one\ntwo\nthree\n"), 2)
	require.NoError(t, err)
	assert.Equal(t, "two\nthree", content)
}

func TestParseMeminfo_SwapIncluded(t *testing.T) {
	t.Parallel()
	svc := system.NewService(t.TempDir(), sampleProcFS(), nil)
	info, err := svc.Info(context.Background())
	require.NoError(t, err)
	require.Len(t, info.Memory, 2)
	assert.Equal(t, "swap", info.Memory[1].Label)
	assert.Equal(t, int64(512000), info.Memory[1].Used)
}

func TestCommandAdapter_ForwardsStdout(t *testing.T) {
	t.Parallel()
	cmd := system.CommandAdapter{Runner: stubCommandRunner{out: "hello"}}
	out, err := cmd.Run(context.Background(), "echo", "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", out)
}

type stubCommandRunner struct {
	out string
}

func (s stubCommandRunner) Run(_ context.Context, _ string, _ ...string) (contracts.CommandResult, error) {
	return contracts.CommandResult{Stdout: s.out}, nil
}

func (s stubCommandRunner) RunWithInput(_ context.Context, _ io.Reader, _ string, _ ...string) (contracts.CommandResult, error) {
	return contracts.CommandResult{Stdout: s.out}, nil
}
