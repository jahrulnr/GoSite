package docker_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/service/docker"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_List_ReturnsContainers(t *testing.T) {
	mock := testutil.NewMockDocker()
	mock.Containers = []contracts.ContainerSummary{
		{ID: "c1", Name: "web", Image: "nginx", Status: "running"},
	}
	svc := docker.NewService(mock)
	rows, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "c1", rows[0].ID)
}

func TestService_List_PropagatesError(t *testing.T) {
	mock := testutil.NewMockDocker()
	mock.ListErr = errors.New("boom")
	svc := docker.NewService(mock)
	_, err := svc.List(context.Background())
	require.Error(t, err)
}

func TestService_Restart_Success(t *testing.T) {
	mock := testutil.NewMockDocker()
	mock.Containers = []contracts.ContainerSummary{{ID: "c1", Status: "running"}}
	svc := docker.NewService(mock)
	require.NoError(t, svc.Restart(context.Background(), "c1"))
	assert.Equal(t, "restarting", mock.Containers[0].Status)
}

func TestService_Restart_InvalidID(t *testing.T) {
	svc := docker.NewService(testutil.NewMockDocker())
	err := svc.Restart(context.Background(), "bad id")
	require.Error(t, err)
	appErr := apperror.From(err)
	assert.Equal(t, apperror.CodeInvalidInput, appErr.Code)
}

func TestService_Stop_Success(t *testing.T) {
	mock := testutil.NewMockDocker()
	mock.Containers = []contracts.ContainerSummary{{ID: "c2", Status: "running"}}
	svc := docker.NewService(mock)
	require.NoError(t, svc.Stop(context.Background(), "c2"))
	assert.Equal(t, "exited", mock.Containers[0].Status)
}

func TestService_Logs_ReturnsText(t *testing.T) {
	mock := testutil.NewMockDocker()
	svc := docker.NewService(mock)
	logs, err := svc.Logs(context.Background(), "c1", 100)
	require.NoError(t, err)
	assert.Contains(t, logs, "c1")
}

func TestService_Logs_InvalidID(t *testing.T) {
	svc := docker.NewService(testutil.NewMockDocker())
	_, err := svc.Logs(context.Background(), "../../x", 10)
	require.Error(t, err)
}

func TestService_Restart_NotFound(t *testing.T) {
	mock := testutil.NewMockDocker()
	svc := docker.NewService(mock)
	err := svc.Restart(context.Background(), "missing")
	require.Error(t, err)
}

func TestService_Stop_InvalidID(t *testing.T) {
	svc := docker.NewService(testutil.NewMockDocker())
	err := svc.Stop(context.Background(), "bad/id")
	require.Error(t, err)
}

func TestService_Logs_DefaultTail(t *testing.T) {
	mock := testutil.NewMockDocker()
	svc := docker.NewService(mock)
	logs, err := svc.Logs(context.Background(), "abc", 0)
	require.NoError(t, err)
	assert.Contains(t, logs, "tail=200")
}
