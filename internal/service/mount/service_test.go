package mount_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jahrulnr/gosite/internal/service/mount"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMountSvc(t *testing.T) (*mount.Service, string, *testutil.MockCommander) {
	t.Helper()
	root := t.TempDir()
	fstab := filepath.Join(root, "fstab")
	cmd := testutil.NewMockCommander()
	return mount.NewService(fstab, cmd), fstab, cmd
}

func TestMount_ListEmpty(t *testing.T) {
	svc, _, _ := newMountSvc(t)
	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestMount_AddAndList(t *testing.T) {
	svc, fstab, _ := newMountSvc(t)
	entry := mount.Entry{Device: "/dev/sdb1", Dir: "/mnt/data", Type: "ext4", Options: "defaults", Dump: "0", Fsck: "0"}
	require.NoError(t, svc.Add(context.Background(), entry))
	data, err := os.ReadFile(fstab)
	require.NoError(t, err)
	assert.Contains(t, string(data), "/dev/sdb1")
	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "/mnt/data", entries[0].Dir)
}

func TestMount_AddDuplicateConflict(t *testing.T) {
	svc, _, _ := newMountSvc(t)
	entry := mount.Entry{Device: "/dev/sdb1", Dir: "/mnt/data", Type: "ext4", Options: "defaults", Dump: "0", Fsck: "0"}
	require.NoError(t, svc.Add(context.Background(), entry))
	err := svc.Add(context.Background(), entry)
	require.Error(t, err)
	assert.Equal(t, apperror.CodeConflict, apperror.From(err).Code)
}

func TestMount_UpdateEntry(t *testing.T) {
	svc, _, _ := newMountSvc(t)
	old := mount.Entry{Device: "/dev/sdb1", Dir: "/mnt/old", Type: "ext4", Options: "defaults", Dump: "0", Fsck: "0"}
	require.NoError(t, svc.Add(context.Background(), old))
	updated := mount.Entry{Device: "/dev/sdb2", Dir: "/mnt/new", Type: "ext4", Options: "defaults", Dump: "0", Fsck: "0"}
	require.NoError(t, svc.Update(context.Background(), old.Device, old.Dir, updated))
	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "/mnt/new", entries[0].Dir)
}

func TestMount_DeleteEntry(t *testing.T) {
	svc, _, cmd := newMountSvc(t)
	entry := mount.Entry{Device: "/dev/sdb1", Dir: "/mnt/data", Type: "ext4", Options: "defaults", Dump: "0", Fsck: "0"}
	require.NoError(t, svc.Add(context.Background(), entry))
	require.NoError(t, svc.Delete(context.Background(), entry.Device, entry.Dir))
	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, entries)
	require.NotEmpty(t, cmd.SnapshotCalls())
}

func TestMount_EnableSuccess(t *testing.T) {
	svc, _, cmd := newMountSvc(t)
	dir := filepath.Join(t.TempDir(), "mnt")
	require.NoError(t, svc.Enable(context.Background(), "/dev/sdb1", dir))
	calls := cmd.SnapshotCalls()
	require.NotEmpty(t, calls)
	assert.Equal(t, "mount", calls[len(calls)-1].Name)
}

func TestMount_EnableFailure(t *testing.T) {
	svc, _, cmd := newMountSvc(t)
	cmd.Stdout = "failed"
	cmd.Err = assert.AnError
	err := svc.Enable(context.Background(), "/dev/sdb1", "/mnt/data")
	require.Error(t, err)
}

func TestMount_UpdateNotFound(t *testing.T) {
	svc, _, _ := newMountSvc(t)
	err := svc.Update(context.Background(), "x", "y", mount.Entry{Device: "a", Dir: "b", Type: "ext4"})
	require.Error(t, err)
	assert.Equal(t, apperror.CodeNotFound, apperror.From(err).Code)
}

func TestMount_DeleteNotFound(t *testing.T) {
	svc, _, _ := newMountSvc(t)
	err := svc.Delete(context.Background(), "x", "y")
	require.Error(t, err)
}

func TestMount_AddMinimalBodyRoundTrip(t *testing.T) {
	svc, _, _ := newMountSvc(t)
	entry := mount.Entry{Device: "/dev/sdb1", Dir: "/mnt/data", Type: "ext4"}
	require.NoError(t, svc.Add(context.Background(), entry))
	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "0", entries[0].Dump)
	assert.Equal(t, "0", entries[0].Fsck)
	assert.Equal(t, "defaults", entries[0].Options)
}

func TestMount_AddMissingDevice(t *testing.T) {
	svc, _, _ := newMountSvc(t)
	err := svc.Add(context.Background(), mount.Entry{Dir: "/mnt/x", Type: "ext4"})
	require.Error(t, err)
}

func TestMount_ListMountedStatus(t *testing.T) {
	svc, _, cmd := newMountSvc(t)
	entry := mount.Entry{Device: "/dev/sdb1", Dir: "/mnt/data", Type: "ext4", Options: "defaults", Dump: "0", Fsck: "0"}
	require.NoError(t, svc.Add(context.Background(), entry))
	cmd.Stdout = "ok"
	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.True(t, entries[0].Mounted)
}

func TestMount_EnableRequiresDir(t *testing.T) {
	svc, _, _ := newMountSvc(t)
	err := svc.Enable(context.Background(), "/dev/sdb1", "")
	require.Error(t, err)
}
