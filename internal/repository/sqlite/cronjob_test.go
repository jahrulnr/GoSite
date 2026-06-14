package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openCronDB(t *testing.T) *sqlite.CronJobRepository {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))
	return sqlite.NewCronJobRepository(db)
}

func TestCronJobRepo_CreateFindList(t *testing.T) {
	repo := openCronDB(t)
	ctx := context.Background()
	created, err := repo.Create(ctx, sqlite.CronJob{
		Name: "renew", Payload: "certbot renew", RunEvery: sqlite.RunEveryDay,
	})
	require.NoError(t, err)
	assert.NotZero(t, created.ID)

	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "renew", found.Name)

	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestCronJobRepo_Update(t *testing.T) {
	repo := openCronDB(t)
	ctx := context.Background()
	job, err := repo.Create(ctx, sqlite.CronJob{
		Name: "old", Payload: "echo old", RunEvery: sqlite.RunEveryHour,
	})
	require.NoError(t, err)
	job.Name = "new"
	job.Payload = "echo new"
	job.RunEvery = sqlite.RunEveryDay
	updated, err := repo.Update(ctx, job)
	require.NoError(t, err)
	assert.Equal(t, "new", updated.Name)
}

func TestCronJobRepo_Delete(t *testing.T) {
	repo := openCronDB(t)
	ctx := context.Background()
	job, err := repo.Create(ctx, sqlite.CronJob{
		Name: "rm", Payload: "echo", RunEvery: sqlite.RunEveryMinute,
	})
	require.NoError(t, err)
	require.NoError(t, repo.Delete(ctx, job.ID))
	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestCronJobRepo_TouchExecutedAt(t *testing.T) {
	repo := openCronDB(t)
	ctx := context.Background()
	job, err := repo.Create(ctx, sqlite.CronJob{
		Name: "touch", Payload: "echo", RunEvery: sqlite.RunEveryDay,
	})
	require.NoError(t, err)
	require.NoError(t, repo.TouchExecutedAt(ctx, job.ID))
	found, err := repo.FindByID(ctx, job.ID)
	require.NoError(t, err)
	assert.NotNil(t, found.ExecutedAt)
}

func TestCronJobRepo_FindByIDMissing(t *testing.T) {
	repo := openCronDB(t)
	_, err := repo.FindByID(context.Background(), 999)
	require.Error(t, err)
}

func TestCronJobRepo_DeleteMissing(t *testing.T) {
	repo := openCronDB(t)
	err := repo.Delete(context.Background(), 999)
	require.Error(t, err)
}

func TestCronJobRepo_ListEmpty(t *testing.T) {
	repo := openCronDB(t)
	list, err := repo.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestCronJobRepo_CreatePreservesRunEvery(t *testing.T) {
	repo := openCronDB(t)
	job, err := repo.Create(context.Background(), sqlite.CronJob{
		Name: "m", Payload: "echo", RunEvery: sqlite.RunEveryMonth,
	})
	require.NoError(t, err)
	assert.Equal(t, sqlite.RunEveryMonth, job.RunEvery)
}

func TestCronJobRepo_UpdatePreservesID(t *testing.T) {
	repo := openCronDB(t)
	ctx := context.Background()
	job, err := repo.Create(ctx, sqlite.CronJob{Name: "a", Payload: "echo", RunEvery: sqlite.RunEveryMinute})
	require.NoError(t, err)
	job.Name = "b"
	updated, err := repo.Update(ctx, job)
	require.NoError(t, err)
	assert.Equal(t, job.ID, updated.ID)
}

func TestCronJobRepo_MultipleRows(t *testing.T) {
	repo := openCronDB(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, err := repo.Create(ctx, sqlite.CronJob{
			Name: "job", Payload: "echo", RunEvery: sqlite.RunEveryDay,
		})
		require.NoError(t, err)
	}
	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 3)
}
