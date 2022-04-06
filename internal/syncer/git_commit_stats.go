package syncer

import (
	"context"
	"database/sql"
	"errors"
	"io/ioutil"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/jackc/pgx/v4"
	"github.com/mergestat/fuse/internal/db"
	uuid "github.com/satori/go.uuid"
)

// sendBatchCommitStats uses the pg COPY protocol to send a batch of commit stats
func (w *worker) sendBatchCommitStats(ctx context.Context, tx pgx.Tx, j *db.DequeueSyncJobRow, batch []*commitStat) error {
	inputs := make([][]interface{}, 0, len(batch))
	for _, c := range batch {
		var repoID uuid.UUID
		var err error
		if repoID, err = uuid.FromString(j.RepoID.String()); err != nil {
			return err
		}
		input := []interface{}{repoID, c.CommitHash.String, c.FilePath.String, c.Additions.Int64, c.Deletions.Int64}
		inputs = append(inputs, input)
	}

	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"git_commit_stats"}, []string{"repo_id", "commit_hash", "file_path", "additions", "deletions"}, pgx.CopyFromRows(inputs)); err != nil {
		return err
	}
	return nil
}

type commitStat struct {
	CommitHash sql.NullString `db:"commit_hash"`
	FilePath   sql.NullString `db:"file_path"`
	Additions  sql.NullInt64  `db:"additions"`
	Deletions  sql.NullInt64  `db:"deletions"`
}

const selectCommitStats = `SELECT commits.hash AS commit_hash, stats.file_path, stats.additions, stats.deletions FROM commits(?), stats(?, commits.hash);`

func (w *worker) handleGitCommitStats(ctx context.Context, j *db.DequeueSyncJobRow) error {
	l := w.loggerForJob(j)

	// TODO(patrickdevivo) uplift the following os.Getenv call to one place, pass value down as a param
	tmpPath, err := ioutil.TempDir(os.Getenv("GIT_CLONE_PATH"), "mergestat-repo-")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(tmpPath); err != nil {
			w.logger.Err(err).Msgf("error cleaning up repo at: %s, %v", tmpPath, err)
		}
	}()

	// TODO figure out this token thing
	if _, err := git.PlainCloneContext(ctx, tmpPath, true, &git.CloneOptions{URL: j.Repo, Auth: &http.BasicAuth{Username: os.Getenv("GITHUB_TOKEN")}}); err != nil {
		return err
	}

	// indicate that we're starting query execution
	if err := w.sendBatchLogMessages(ctx, []*syncLog{
		{Type: SyncLogTypeInfo, RepoSyncQueueID: j.ID, Message: "starting to execute commit stats query"},
	}); err != nil {
		return err
	}

	// execute mergestat query
	// even for large repos, we should be able to hold all commits in mem easily?
	stats := make([]*commitStat, 0)
	if err = w.mergestat.SelectContext(ctx, &stats, selectCommitStats, tmpPath, tmpPath); err != nil {
		return err
	}

	l.Info().Msgf("retrieved stats: %d", len(stats))

	var tx pgx.Tx
	if tx, err = w.pool.BeginTx(ctx, pgx.TxOptions{}); err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			if !errors.Is(err, pgx.ErrTxClosed) {
				w.logger.Err(err).Msgf("could not rollback transaction")
			}
		}
	}()

	if _, err := tx.Exec(ctx, "DELETE FROM git_commit_stats WHERE repo_id = $1;", j.RepoID.String()); err != nil {
		return err
	}

	if err := w.sendBatchCommitStats(ctx, tx, j, stats); err != nil {
		return err
	}

	l.Info().Msgf("sent batch of %d commit stats", len(stats))

	if err := w.db.WithTx(tx).SetSyncJobStatus(ctx, db.SetSyncJobStatusParams{Status: "DONE", ID: j.ID}); err != nil {
		return err
	}

	if err := w.sendBatchLogMessages(ctx, []*syncLog{
		{Type: SyncLogTypeInfo, RepoSyncQueueID: j.ID, Message: "finished!"},
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
