package files_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jahrulnr/gosite/internal/service/files"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFilesSvc(t *testing.T, allowExecute bool) (*files.Service, string) {
	t.Helper()
	root := t.TempDir()
	www := filepath.Join(root, "www")
	storage := filepath.Join(root, "storage")
	tmp := filepath.Join(root, "tmp")
	require.NoError(t, os.MkdirAll(www, 0o755))
	require.NoError(t, os.MkdirAll(storage, 0o755))
	require.NoError(t, os.MkdirAll(tmp, 0o755))
	svc := files.NewService([]string{www, storage, tmp}, allowExecute, testutil.NewMockCommander())
	return svc, www
}

func TestFiles_PathTraversalRejected(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	traversal := www + "/site/../../etc/passwd"
	_, err := svc.Browse(context.Background(), traversal)
	require.Error(t, err)
	assert.Equal(t, apperror.CodePathTraversal, apperror.From(err).Code)
}

func TestFiles_ExecuteDisabledByDefault(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	script := filepath.Join(www, "run.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh"), 0o755))
	err := svc.Action(context.Background(), files.ActionInput{Action: "execute", Path: script})
	require.Error(t, err)
	assert.Equal(t, apperror.CodeFileExecuteDisabled, apperror.From(err).Code)
}

func TestFiles_BrowseListsDirectory(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	require.NoError(t, os.WriteFile(filepath.Join(www, "a.txt"), []byte("a"), 0o644))
	entries, err := svc.Browse(context.Background(), www)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "a.txt", entries[0].Name)
}

func TestFiles_ReadFileContent(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	path := filepath.Join(www, "hello.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o644))
	content, err := svc.Read(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, "hello", content)
}

func TestFiles_CreateDirectory(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	err := svc.Create(context.Background(), files.CreateInput{Type: "directory", Name: "nested", Path: www})
	require.NoError(t, err)
	info, err := os.Stat(filepath.Join(www, "nested"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestFiles_CreateFile(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	err := svc.Create(context.Background(), files.CreateInput{
		Type: "file", Name: "new.txt", Path: www, Content: "data",
	})
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(www, "new.txt"))
	require.NoError(t, err)
	assert.Equal(t, "data", string(data))
}

func TestFiles_Upload(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	err := svc.Upload(context.Background(), www, "up.txt", strings.NewReader("uploaded"))
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(www, "up.txt"))
	require.NoError(t, err)
	assert.Equal(t, "uploaded", string(data))
}

func TestFiles_Chmod(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	path := filepath.Join(www, "perm.txt")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))
	require.NoError(t, svc.Action(context.Background(), files.ActionInput{Action: "chmod", Path: path, Mode: "600"}))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestFiles_Copy(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	src := filepath.Join(www, "src.txt")
	dst := filepath.Join(www, "dst.txt")
	require.NoError(t, os.WriteFile(src, []byte("copy"), 0o644))
	require.NoError(t, svc.Action(context.Background(), files.ActionInput{Action: "copy", Path: src, ToPath: dst}))
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "copy", string(data))
}

func TestFiles_Delete(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	path := filepath.Join(www, "gone.txt")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))
	require.NoError(t, svc.Delete(context.Background(), path))
	_, err := os.Stat(path)
	require.Error(t, err)
}

func TestFiles_ExecuteEnabled(t *testing.T) {
	root := t.TempDir()
	cmd := testutil.NewMockCommander()
	svc := files.NewService([]string{root}, true, cmd)
	script := filepath.Join(root, "run.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh"), 0o755))
	require.NoError(t, svc.Action(context.Background(), files.ActionInput{Action: "execute", Path: script}))
	assert.NotEmpty(t, cmd.SnapshotCalls())
}

func TestFiles_BrowseRejectsFilePath(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	file := filepath.Join(www, "only.txt")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))
	_, err := svc.Browse(context.Background(), file)
	require.Error(t, err)
	assert.Equal(t, apperror.CodePathIsFile, apperror.From(err).Code)
}

func TestFiles_ReadRejectsDirectory(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	_, err := svc.Read(context.Background(), www)
	require.Error(t, err)
}

func TestFiles_CreateInvalidType(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	err := svc.Create(context.Background(), files.CreateInput{Type: "socket", Name: "x", Path: www})
	require.Error(t, err)
}

func TestFiles_UploadEmptyFilename(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	err := svc.Upload(context.Background(), www, "", strings.NewReader("x"))
	require.Error(t, err)
}

func TestFiles_ChmodInvalidMode(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	path := filepath.Join(www, "mode.txt")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))
	err := svc.Action(context.Background(), files.ActionInput{Action: "chmod", Path: path, Mode: "bad"})
	require.Error(t, err)
}

func TestFiles_InvalidAction(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	err := svc.Action(context.Background(), files.ActionInput{Action: "rename", Path: www})
	require.Error(t, err)
}

func TestFiles_ReadMissingFile(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	_, err := svc.Read(context.Background(), filepath.Join(www, "missing.txt"))
	require.Error(t, err)
}

func TestFiles_CopySourceMissing(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	err := svc.Action(context.Background(), files.ActionInput{
		Action: "copy", Path: filepath.Join(www, "missing.txt"), ToPath: filepath.Join(www, "dst.txt"),
	})
	require.Error(t, err)
}

func TestFiles_CopySourceIsDirectory(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	sub := filepath.Join(www, "subdir")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	err := svc.Action(context.Background(), files.ActionInput{
		Action: "copy", Path: sub, ToPath: filepath.Join(www, "out.txt"),
	})
	require.Error(t, err)
}

func TestFiles_ExecuteDirectoryRejected(t *testing.T) {
	root := t.TempDir()
	cmd := testutil.NewMockCommander()
	svc := files.NewService([]string{root}, true, cmd)
	err := svc.Action(context.Background(), files.ActionInput{Action: "execute", Path: root})
	require.Error(t, err)
}

func TestFiles_CreateNameTraversalRejected(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	err := svc.Create(context.Background(), files.CreateInput{Type: "file", Name: "../escape", Path: www})
	require.Error(t, err)
}

func TestFiles_UploadTraversalRejected(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	err := svc.Upload(context.Background(), www, "../evil.txt", strings.NewReader("x"))
	require.Error(t, err)
}

func TestFiles_ChmodEmptyMode(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	path := filepath.Join(www, "empty-mode.txt")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))
	err := svc.Action(context.Background(), files.ActionInput{Action: "chmod", Path: path})
	require.Error(t, err)
}

func TestFiles_CreateMissingName(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	err := svc.Create(context.Background(), files.CreateInput{Type: "file", Path: www})
	require.Error(t, err)
}

func TestFiles_ChmodOutOfRange(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	path := filepath.Join(www, "range.txt")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))
	err := svc.Action(context.Background(), files.ActionInput{Action: "chmod", Path: path, Mode: "999"})
	require.Error(t, err)
}

func TestFiles_BrowseNestedDirectory(t *testing.T) {
	svc, www := newFilesSvc(t, false)
	nested := filepath.Join(www, "a", "b")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "deep.txt"), []byte("d"), 0o644))
	entries, err := svc.Browse(context.Background(), nested)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestFiles_ExecuteCommandFailure(t *testing.T) {
	root := t.TempDir()
	cmd := testutil.NewMockCommander()
	cmd.Err = assert.AnError
	svc := files.NewService([]string{root}, true, cmd)
	script := filepath.Join(root, "fail.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh"), 0o755))
	err := svc.Action(context.Background(), files.ActionInput{Action: "execute", Path: script})
	require.Error(t, err)
}

func TestFiles_DeleteInvalidPath(t *testing.T) {
	svc, _ := newFilesSvc(t, false)
	err := svc.Delete(context.Background(), "/etc/passwd")
	require.Error(t, err)
}
