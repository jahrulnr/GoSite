package mount_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jahrulnr/gosite/internal/service/mount"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3_AddAndList(t *testing.T) {
	root := t.TempDir()
	fstab := filepath.Join(root, "fstab")
	secrets := filepath.Join(root, "secrets")
	cmd := testutil.NewMockCommander()
	svc := mount.NewService(fstab, secrets, cmd)

	entry := mount.Entry{
		Dir:  "/storage/mnt/s3-backup",
		Type: "fuse.s3fs",
		S3: &mount.S3Config{
			Bucket:    "my-site-assets",
			Endpoint:  "https://s3.ap-southeast-1.amazonaws.com",
			Region:    "ap-southeast-1",
			AccessKey: "AKIAEXAMPLE",
			SecretKey: "secret-example",
			PathStyle: false,
		},
	}
	require.NoError(t, svc.Add(context.Background(), entry))

	data, err := os.ReadFile(fstab)
	require.NoError(t, err)
	line := string(data)
	assert.Contains(t, line, "my-site-assets")
	assert.Contains(t, line, "/storage/mnt/s3-backup")
	assert.Contains(t, line, "fuse.s3fs")
	assert.Contains(t, line, "passwd_file=")

	passwdFiles, err := os.ReadDir(secrets)
	require.NoError(t, err)
	require.Len(t, passwdFiles, 1)
	passwdData, err := os.ReadFile(filepath.Join(secrets, passwdFiles[0].Name()))
	require.NoError(t, err)
	assert.Equal(t, "AKIAEXAMPLE:secret-example\n", string(passwdData))

	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "my-site-assets", entries[0].Device)
	require.NotNil(t, entries[0].S3)
	assert.Equal(t, "my-site-assets", entries[0].S3.Bucket)
	assert.Equal(t, "", entries[0].S3.AccessKey)
}

func TestS3_RequiresCredentialsOnCreate(t *testing.T) {
	root := t.TempDir()
	svc := mount.NewService(filepath.Join(root, "fstab"), filepath.Join(root, "secrets"), testutil.NewMockCommander())
	err := svc.Add(context.Background(), mount.Entry{
		Dir:  "/storage/mnt/s3",
		Type: "s3",
		S3:   &mount.S3Config{Bucket: "valid-bucket"},
	})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "access key")
}

func TestS3_UpdateKeepsCredentials(t *testing.T) {
	root := t.TempDir()
	fstab := filepath.Join(root, "fstab")
	secrets := filepath.Join(root, "secrets")
	svc := mount.NewService(fstab, secrets, testutil.NewMockCommander())

	original := mount.Entry{
		Dir:  "/storage/mnt/s3-data",
		Type: "fuse.s3fs",
		S3: &mount.S3Config{
			Bucket:    "site-backup",
			AccessKey: "KEY1",
			SecretKey: "SECRET1",
		},
	}
	require.NoError(t, svc.Add(context.Background(), original))

	updated := mount.Entry{
		Dir:  "/storage/mnt/s3-data",
		Type: "fuse.s3fs",
		S3: &mount.S3Config{
			Bucket:   "site-backup",
			Endpoint: "https://minio.example.com",
			PathStyle: true,
		},
	}
	require.NoError(t, svc.Update(context.Background(), "site-backup", original.Dir, updated))

	data, err := os.ReadFile(fstab)
	require.NoError(t, err)
	assert.Contains(t, string(data), "url=https://minio.example.com")
	assert.Contains(t, string(data), "use_path_request_style")

	passwdFiles, err := os.ReadDir(secrets)
	require.NoError(t, err)
	require.Len(t, passwdFiles, 1)
	passwdData, err := os.ReadFile(filepath.Join(secrets, passwdFiles[0].Name()))
	require.NoError(t, err)
	assert.Equal(t, "KEY1:SECRET1\n", string(passwdData))
}

func TestS3_DeleteRemovesPasswd(t *testing.T) {
	root := t.TempDir()
	fstab := filepath.Join(root, "fstab")
	secrets := filepath.Join(root, "secrets")
	svc := mount.NewService(fstab, secrets, testutil.NewMockCommander())

	entry := mount.Entry{
		Dir:  "/storage/mnt/s3-temp",
		Type: "fuse.s3fs",
		S3: &mount.S3Config{
			Bucket:    "temp-bucket",
			AccessKey: "KEY",
			SecretKey: "SECRET",
		},
	}
	require.NoError(t, svc.Add(context.Background(), entry))
	require.NoError(t, svc.Delete(context.Background(), "temp-bucket", entry.Dir))
	_, err := os.ReadDir(secrets)
	require.NoError(t, err)
	files, _ := os.ReadDir(secrets)
	assert.Empty(t, files)
}
