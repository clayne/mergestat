// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.16.0
// source: queries.sql

package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
)

const cleanOldRepoSyncQueue = `-- name: CleanOldRepoSyncQueue :exec
SELECT mergestat.simple_repo_sync_queue_cleanup($1::INTEGER)
`

func (q *Queries) CleanOldRepoSyncQueue(ctx context.Context, dollar_1 int32) error {
	_, err := q.db.Exec(ctx, cleanOldRepoSyncQueue, dollar_1)
	return err
}

const deleteGitHubRepoInfo = `-- name: DeleteGitHubRepoInfo :exec
DELETE FROM public.github_repo_info WHERE repo_id = $1
`

func (q *Queries) DeleteGitHubRepoInfo(ctx context.Context, repoID uuid.UUID) error {
	_, err := q.db.Exec(ctx, deleteGitHubRepoInfo, repoID)
	return err
}

const deleteRemovedRepos = `-- name: DeleteRemovedRepos :exec
DELETE FROM public.repos WHERE repo_import_id = $1::uuid AND NOT(repo = ANY($2::TEXT[]))
`

type DeleteRemovedReposParams struct {
	Column1 uuid.UUID
	Column2 []string
}

func (q *Queries) DeleteRemovedRepos(ctx context.Context, arg DeleteRemovedReposParams) error {
	_, err := q.db.Exec(ctx, deleteRemovedRepos, arg.Column1, arg.Column2)
	return err
}

const dequeueSyncJob = `-- name: DequeueSyncJob :one
WITH
running AS (
        SELECT 
            rsq.id,
            rstg.group
        FROM mergestat.repo_sync_queue rsq
        INNER JOIN mergestat.repo_sync_type_groups rstg ON rsq.type_group = rstg.group
        WHERE status = 'RUNNING'
),
dequeued AS (
   UPDATE mergestat.repo_sync_queue SET status = 'RUNNING'
   WHERE id IN (   
        SELECT rsq.id
        FROM mergestat.repo_sync_queue rsq
        INNER JOIN mergestat.repo_sync_type_groups rstg ON rsq.type_group = rstg.group
        WHERE status = 'QUEUED'
        AND rstg.concurrent_syncs > (SELECT COUNT(*) FROM running WHERE running.group = rstg.group)
        ORDER BY rsq.priority ASC, rsq.created_at ASC, rsq.id ASC LIMIT 1 FOR UPDATE SKIP LOCKED
   ) RETURNING id, created_at, status, repo_sync_id
)
SELECT
    dequeued.id, dequeued.created_at, dequeued.status, dequeued.repo_sync_id,
    repo_syncs.repo_id, repo_syncs.sync_type, repo_syncs.settings, repo_syncs.id, repo_syncs.schedule_enabled, repo_syncs.priority, repo_syncs.last_completed_repo_sync_queue_id,
    repos.repo,
    repos.ref,
    repos.settings AS repo_settings
FROM dequeued
JOIN mergestat.repo_syncs ON mergestat.repo_syncs.id = dequeued.repo_sync_id
JOIN repos ON repos.id = mergestat.repo_syncs.repo_id
`

type DequeueSyncJobRow struct {
	ID                           int64
	CreatedAt                    time.Time
	Status                       string
	RepoSyncID                   uuid.UUID
	RepoID                       uuid.UUID
	SyncType                     string
	Settings                     pgtype.JSONB
	ID_2                         uuid.UUID
	ScheduleEnabled              bool
	Priority                     int32
	LastCompletedRepoSyncQueueID sql.NullInt64
	Repo                         string
	Ref                          sql.NullString
	RepoSettings                 pgtype.JSONB
}

func (q *Queries) DequeueSyncJob(ctx context.Context) (DequeueSyncJobRow, error) {
	row := q.db.QueryRow(ctx, dequeueSyncJob)
	var i DequeueSyncJobRow
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.Status,
		&i.RepoSyncID,
		&i.RepoID,
		&i.SyncType,
		&i.Settings,
		&i.ID_2,
		&i.ScheduleEnabled,
		&i.Priority,
		&i.LastCompletedRepoSyncQueueID,
		&i.Repo,
		&i.Ref,
		&i.RepoSettings,
	)
	return i, err
}

const enqueueAllSyncs = `-- name: EnqueueAllSyncs :exec
WITH ranked_queue AS (
    SELECT
       rsq.done_at,
       rst.type_group,
       rsq.created_at,
       DENSE_RANK() OVER(PARTITION BY rst.type_group ORDER BY rst.type_group, rsq.created_at DESC) AS rank_num
    FROM mergestat.repo_syncs as rs
    INNER JOIN mergestat.repo_sync_queue AS rsq ON rs.id = rsq.repo_sync_id
    INNER JOIN mergestat.repo_sync_types AS rst ON rs.sync_type = rst.type
    WHERE rsq.done_at IS NULL
)
INSERT INTO mergestat.repo_sync_queue (repo_sync_id, status, priority, type_group)
SELECT
    rs.id,
    'QUEUED' AS status,
	rs.priority,
    rst.type_group
FROM mergestat.repo_syncs rs
INNER JOIN mergestat.repo_sync_types AS rst ON rs.sync_type = rst.type
WHERE schedule_enabled
    AND id NOT IN (SELECT repo_sync_id FROM mergestat.repo_sync_queue WHERE status = 'RUNNING' OR status = 'QUEUED')
    AND NOT EXISTS (
        SELECT rq.done_at
        FROM ranked_queue rq
        WHERE
            rq.rank_num >= 1
	AND rq.type_group = rst.type_group
    )
ORDER BY rs.priority, rs.sync_type desc
`

// We use a CTE here to retrieve all the repo_sync_jobs that were previously enqueued, to make sure that we *do not* re-enqueue anything new until the previously enqueued jobs are *completed*.
// This allows us to make sure all repo syncs complete before we reschedule a new batch.
// We have now also added a concept of type groups which allows us to apply this same logic but by each group type which is where the PARTITION BY clause comes into play
func (q *Queries) EnqueueAllSyncs(ctx context.Context) error {
	_, err := q.db.Exec(ctx, enqueueAllSyncs)
	return err
}

const fetchGitHubToken = `-- name: FetchGitHubToken :one
SELECT pgp_sym_decrypt(credentials, $1) FROM mergestat.service_auth_credentials WHERE type = 'GITHUB_PAT' ORDER BY created_at DESC LIMIT 1
`

func (q *Queries) FetchGitHubToken(ctx context.Context, pgpSymDecrypt string) (string, error) {
	row := q.db.QueryRow(ctx, fetchGitHubToken, pgpSymDecrypt)
	var pgp_sym_decrypt string
	err := row.Scan(&pgp_sym_decrypt)
	return pgp_sym_decrypt, err
}

const getRepoById = `-- name: GetRepoById :one
SELECT id, repo, ref, created_at, settings, tags, repo_import_id, provider FROM public.repos WHERE id = $1
`

func (q *Queries) GetRepoById(ctx context.Context, id uuid.UUID) (Repo, error) {
	row := q.db.QueryRow(ctx, getRepoById, id)
	var i Repo
	err := row.Scan(
		&i.ID,
		&i.Repo,
		&i.Ref,
		&i.CreatedAt,
		&i.Settings,
		&i.Tags,
		&i.RepoImportID,
		&i.Provider,
	)
	return i, err
}

const getRepoIDsFromRepoImport = `-- name: GetRepoIDsFromRepoImport :many
SELECT id FROM public.repos WHERE repo_import_id = $1::uuid AND repo = ANY($2::TEXT[])
`

type GetRepoIDsFromRepoImportParams struct {
	Importid  uuid.UUID
	Reposurls []string
}

func (q *Queries) GetRepoIDsFromRepoImport(ctx context.Context, arg GetRepoIDsFromRepoImportParams) ([]uuid.UUID, error) {
	rows, err := q.db.Query(ctx, getRepoIDsFromRepoImport, arg.Importid, arg.Reposurls)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getRepoImportByID = `-- name: GetRepoImportByID :one
SELECT id, created_at, updated_at, settings, last_import, import_interval, last_import_started_at, import_status, import_error, provider FROM mergestat.repo_imports
WHERE id = $1 LIMIT 1
`

func (q *Queries) GetRepoImportByID(ctx context.Context, id uuid.UUID) (MergestatRepoImport, error) {
	row := q.db.QueryRow(ctx, getRepoImportByID, id)
	var i MergestatRepoImport
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Settings,
		&i.LastImport,
		&i.ImportInterval,
		&i.LastImportStartedAt,
		&i.ImportStatus,
		&i.ImportError,
		&i.Provider,
	)
	return i, err
}

const getRepoUrlFromImport = `-- name: GetRepoUrlFromImport :many
SELECT repo FROM public.repos WHERE repo_import_id = $1::uuid
`

func (q *Queries) GetRepoUrlFromImport(ctx context.Context, importid uuid.UUID) ([]string, error) {
	rows, err := q.db.Query(ctx, getRepoUrlFromImport, importid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []string
	for rows.Next() {
		var repo string
		if err := rows.Scan(&repo); err != nil {
			return nil, err
		}
		items = append(items, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const insertGitHubRepoInfo = `-- name: InsertGitHubRepoInfo :exec
INSERT INTO public.github_repo_info (
    repo_id, owner, name,
    created_at, default_branch_name, description, disk_usage, fork_count, homepage_url,
    is_archived, is_disabled, is_mirror, is_private, total_issues_count, latest_release_author,
    latest_release_created_at, latest_release_name, latest_release_published_at, license_key,
    license_name, license_nickname, open_graph_image_url, primary_language, pushed_at, releases_count,
    stargazers_count, updated_at, watchers_count
) VALUES(
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22,
    $23, $24, $25, $26, $27, $28
)
`

type InsertGitHubRepoInfoParams struct {
	RepoID                   uuid.UUID
	Owner                    string
	Name                     string
	CreatedAt                sql.NullTime
	DefaultBranchName        sql.NullString
	Description              sql.NullString
	DiskUsage                sql.NullInt32
	ForkCount                sql.NullInt32
	HomepageUrl              sql.NullString
	IsArchived               sql.NullBool
	IsDisabled               sql.NullBool
	IsMirror                 sql.NullBool
	IsPrivate                sql.NullBool
	TotalIssuesCount         sql.NullInt32
	LatestReleaseAuthor      sql.NullString
	LatestReleaseCreatedAt   sql.NullTime
	LatestReleaseName        sql.NullString
	LatestReleasePublishedAt sql.NullTime
	LicenseKey               sql.NullString
	LicenseName              sql.NullString
	LicenseNickname          sql.NullString
	OpenGraphImageUrl        sql.NullString
	PrimaryLanguage          sql.NullString
	PushedAt                 sql.NullTime
	ReleasesCount            sql.NullInt32
	StargazersCount          sql.NullInt32
	UpdatedAt                sql.NullTime
	WatchersCount            sql.NullInt32
}

func (q *Queries) InsertGitHubRepoInfo(ctx context.Context, arg InsertGitHubRepoInfoParams) error {
	_, err := q.db.Exec(ctx, insertGitHubRepoInfo,
		arg.RepoID,
		arg.Owner,
		arg.Name,
		arg.CreatedAt,
		arg.DefaultBranchName,
		arg.Description,
		arg.DiskUsage,
		arg.ForkCount,
		arg.HomepageUrl,
		arg.IsArchived,
		arg.IsDisabled,
		arg.IsMirror,
		arg.IsPrivate,
		arg.TotalIssuesCount,
		arg.LatestReleaseAuthor,
		arg.LatestReleaseCreatedAt,
		arg.LatestReleaseName,
		arg.LatestReleasePublishedAt,
		arg.LicenseKey,
		arg.LicenseName,
		arg.LicenseNickname,
		arg.OpenGraphImageUrl,
		arg.PrimaryLanguage,
		arg.PushedAt,
		arg.ReleasesCount,
		arg.StargazersCount,
		arg.UpdatedAt,
		arg.WatchersCount,
	)
	return err
}

const insertNewDefaultSync = `-- name: InsertNewDefaultSync :exec
INSERT INTO mergestat.repo_syncs (repo_id, sync_type, priority, schedule_enabled)
SELECT $1::uuid, type, priority, true
FROM mergestat.repo_sync_types
WHERE type = $2::text
ON CONFLICT DO NOTHING
`

type InsertNewDefaultSyncParams struct {
	Repoid   uuid.UUID
	Synctype string
}

func (q *Queries) InsertNewDefaultSync(ctx context.Context, arg InsertNewDefaultSyncParams) error {
	_, err := q.db.Exec(ctx, insertNewDefaultSync, arg.Repoid, arg.Synctype)
	return err
}

const insertSyncJobLog = `-- name: InsertSyncJobLog :exec
INSERT INTO mergestat.repo_sync_logs (log_type, message, repo_sync_queue_id) VALUES ($1, $2, $3)
`

type InsertSyncJobLogParams struct {
	LogType         string
	Message         string
	RepoSyncQueueID int64
}

func (q *Queries) InsertSyncJobLog(ctx context.Context, arg InsertSyncJobLogParams) error {
	_, err := q.db.Exec(ctx, insertSyncJobLog, arg.LogType, arg.Message, arg.RepoSyncQueueID)
	return err
}

const listRepoImportsDueForImport = `-- name: ListRepoImportsDueForImport :many
WITH dequeued AS (
    UPDATE mergestat.repo_imports SET last_import_started_at = now()
    WHERE id IN (
        SELECT id FROM mergestat.repo_imports AS t
        WHERE
            (now() - t.last_import > t.import_interval OR t.last_import IS NULL)
            AND
            (now() - t.last_import_started_at > t.import_interval OR t.last_import_started_at IS NULL)
        ORDER BY last_import ASC
        FOR UPDATE SKIP LOCKED
    ) RETURNING id, created_at, updated_at, settings, last_import, import_interval, last_import_started_at, import_status, import_error, provider
)
SELECT dq.id, dq.created_at, dq.updated_at, dq.settings, dq.provider, pr.settings AS provider_settings, vd.name AS vendor_name
FROM dequeued dq
    INNER JOIN mergestat.providers pr ON pr.id = dq.provider
    INNER JOIN mergestat.vendors vd ON vd.name = pr.vendor
`

type ListRepoImportsDueForImportRow struct {
	ID               uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Settings         pgtype.JSONB
	Provider         uuid.UUID
	ProviderSettings pgtype.JSONB
	VendorName       string
}

func (q *Queries) ListRepoImportsDueForImport(ctx context.Context) ([]ListRepoImportsDueForImportRow, error) {
	rows, err := q.db.Query(ctx, listRepoImportsDueForImport)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ListRepoImportsDueForImportRow
	for rows.Next() {
		var i ListRepoImportsDueForImportRow
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.Settings,
			&i.Provider,
			&i.ProviderSettings,
			&i.VendorName,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const markRepoImportAsUpdated = `-- name: MarkRepoImportAsUpdated :exec
UPDATE mergestat.repo_imports SET last_import = now() WHERE id = $1
`

func (q *Queries) MarkRepoImportAsUpdated(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.Exec(ctx, markRepoImportAsUpdated, id)
	return err
}

const markSyncsAsTimedOut = `-- name: MarkSyncsAsTimedOut :many
WITH timed_out_sync_jobs AS (
    UPDATE mergestat.repo_sync_queue SET status = 'DONE' WHERE status = 'RUNNING' AND (
        (last_keep_alive < now() - '10 minutes'::interval)
        OR
        (last_keep_alive IS NULL AND started_at < now() - '10 minutes'::interval)) -- if worker crashed before last_keep_alive was first set
    RETURNING id, created_at, repo_sync_id, status, started_at, done_at, last_keep_alive, priority, type_group
)
INSERT INTO mergestat.repo_sync_logs (repo_sync_queue_id, log_type, message)
SELECT id, 'ERROR', 'No response from job within reasonable interval. Timing out.' FROM timed_out_sync_jobs
RETURNING repo_sync_queue_id
`

func (q *Queries) MarkSyncsAsTimedOut(ctx context.Context) ([]int64, error) {
	rows, err := q.db.Query(ctx, markSyncsAsTimedOut)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []int64
	for rows.Next() {
		var repo_sync_queue_id int64
		if err := rows.Scan(&repo_sync_queue_id); err != nil {
			return nil, err
		}
		items = append(items, repo_sync_queue_id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const setLatestKeepAliveForJob = `-- name: SetLatestKeepAliveForJob :exec
UPDATE mergestat.repo_sync_queue SET last_keep_alive = now() WHERE id = $1
`

func (q *Queries) SetLatestKeepAliveForJob(ctx context.Context, id int64) error {
	_, err := q.db.Exec(ctx, setLatestKeepAliveForJob, id)
	return err
}

const setSyncJobStatus = `-- name: SetSyncJobStatus :exec
SELECT mergestat.set_sync_job_status($1::TEXT, $2::BIGINT)
`

type SetSyncJobStatusParams struct {
	Status string
	ID     int64
}

func (q *Queries) SetSyncJobStatus(ctx context.Context, arg SetSyncJobStatusParams) error {
	_, err := q.db.Exec(ctx, setSyncJobStatus, arg.Status, arg.ID)
	return err
}

const updateImportStatus = `-- name: UpdateImportStatus :exec
UPDATE mergestat.repo_imports SET import_status = $1::TEXT, import_error = $2::TEXT WHERE id = $3
`

type UpdateImportStatusParams struct {
	Status string
	Error  string
	ID     uuid.UUID
}

func (q *Queries) UpdateImportStatus(ctx context.Context, arg UpdateImportStatusParams) error {
	_, err := q.db.Exec(ctx, updateImportStatus, arg.Status, arg.Error, arg.ID)
	return err
}

const upsertRepo = `-- name: UpsertRepo :exec
INSERT INTO public.repos (repo, repo_import_id, provider) VALUES($1, $2, $3)
ON CONFLICT (repo, (ref IS NULL)) WHERE ref IS NULL
DO UPDATE SET tags = (
    SELECT COALESCE(jsonb_agg(DISTINCT x), jsonb_build_array()) FROM jsonb_array_elements(repos.tags || $4) x LIMIT 1
)
`

type UpsertRepoParams struct {
	Repo         string
	RepoImportID uuid.NullUUID
	Provider     uuid.UUID
	Tags         pgtype.JSONB
}

func (q *Queries) UpsertRepo(ctx context.Context, arg UpsertRepoParams) error {
	_, err := q.db.Exec(ctx, upsertRepo,
		arg.Repo,
		arg.RepoImportID,
		arg.Provider,
		arg.Tags,
	)
	return err
}

const upsertWorkflowRunJobs = `-- name: UpsertWorkflowRunJobs :exec
WITH t AS (
	INSERT INTO public.github_actions_workflow_run_jobs (
		repo_id,
		id,
		run_id,
		log,
		run_url,
		job_node_id,
		head_sha,
		url,
		html_url,
		status,
		conclusion,
		started_at,
		completed_at,
		workflow_name,
		steps,
		check_run_url,
		labels,
		runner_id,
		runner_name,
		runner_group_id,
		runner_group_name
	)
	VALUES(
		$1::uuid,
		$2::BIGINT,
		$3,
		$4,
		$5,
		$6,
		$7,
		$8,
		$9,
		$10,
		$11,
		$12,
		$13,
		$14,
		$15::JSONB,
		$16,
		$17::JSONB,
		$18,
		$19,
		$20,
		$21)
		ON CONFLICT (id)
		DO UPDATE 
		SET repo_id=EXCLUDED.repo_id,
		    id=EXCLUDED.id,
			run_id=EXCLUDED.run_id,
			log=EXCLUDED.log,
			run_url=EXCLUDED.run_url,
			job_node_id=EXCLUDED.job_node_id,
			head_sha=EXCLUDED.head_sha,
			url=EXCLUDED.url,
			html_url=EXCLUDED.html_url,
			status=EXCLUDED.status,
			conclusion=EXCLUDED.conclusion,
			started_at=EXCLUDED.started_at,
			completed_at=EXCLUDED.completed_at,
			workflow_name=EXCLUDED.workflow_name,
			steps=excluded.steps,
			check_run_url=EXCLUDED.check_run_url,
			labels=EXCLUDED.labels,
			runner_id=EXCLUDED.runner_id,
			runner_name=EXCLUDED.runner_name,
			runner_group_id=EXCLUDED.runner_group_id,
			runner_group_name=EXCLUDED.runner_group_name
		RETURNING xmax::text
)
SELECT
    COUNT(*) AS all_rows,
    SUM(CASE WHEN xmax::int = 0 THEN 1 ELSE 0 END) AS ins,
    SUM(CASE WHEN xmax::int > 0 THEN 1 ELSE 0 END) AS upd
FROM t
`

type UpsertWorkflowRunJobsParams struct {
	Repoid          uuid.UUID
	ID              int64
	Runid           sql.NullInt64
	Log             sql.NullString
	Runurl          sql.NullString
	Jobnodeid       sql.NullString
	Headsha         sql.NullString
	Url             sql.NullString
	Htmlurl         sql.NullString
	Status          sql.NullString
	Conclusion      sql.NullString
	Startedat       sql.NullTime
	Completedat     sql.NullTime
	Workflowname    sql.NullString
	Steps           pgtype.JSONB
	Checkrunurl     sql.NullString
	Labels          pgtype.JSONB
	Runnerid        sql.NullInt64
	Runnername      sql.NullString
	Runnergroupid   sql.NullInt64
	Runnergroupname sql.NullString
}

type UpsertWorkflowRunJobsRow struct {
	AllRows int64
	Ins     int64
	Upd     int64
}

func (q *Queries) UpsertWorkflowRunJobs(ctx context.Context, arg UpsertWorkflowRunJobsParams) error {
	_, err := q.db.Exec(ctx, upsertWorkflowRunJobs,
		arg.Repoid,
		arg.ID,
		arg.Runid,
		arg.Log,
		arg.Runurl,
		arg.Jobnodeid,
		arg.Headsha,
		arg.Url,
		arg.Htmlurl,
		arg.Status,
		arg.Conclusion,
		arg.Startedat,
		arg.Completedat,
		arg.Workflowname,
		arg.Steps,
		arg.Checkrunurl,
		arg.Labels,
		arg.Runnerid,
		arg.Runnername,
		arg.Runnergroupid,
		arg.Runnergroupname,
	)
	return err
}

const upsertWorkflowRuns = `-- name: UpsertWorkflowRuns :exec
WITH t AS(
	INSERT INTO public.github_actions_workflow_runs(
	repo_id,
	id,
	workflow_run_node_id,
	name,
	head_branch,
	run_number,
	run_attempt,
	event,
	status,
	conclusion,
	workflow_id,
	check_suite_id,
	check_suite_node_id,
	url,
	html_url,
	pull_requests,
	created_at,
	updated_at,
	run_started_at,
	jobs_url,
	logs_url,
	check_suite_url,
	artifacts_url,
	cancel_url,
	rerun_url,
	head_commit,
	workflow_url,
	repository_url,
	head_repository_url)
	VALUES(
 	$1::UUID,
	$2,
	$3,
    $4,
	$5,
	$6,
	$7,
	$8,
	$9,
	$10,
	$11,
	$12,
	$13,
	$14,
	$15,
	$16::JSONB,
	$17,
	$18,
	$19,
	$20,
	$21,
	$22,
	$23,
	$24,
	$25,
	$26::JSONB,
	$27,
	$28,
	$29)
	ON CONFLICT (id)
    DO UPDATE
    SET repo_id=EXCLUDED.repo_id,
        id=EXCLUDED.id,
		workflow_run_node_id=EXCLUDED.workflow_run_node_id,
		name=EXCLUDED.name,
		head_branch=EXCLUDED.head_branch,
		run_number=EXCLUDED.run_number,
		run_attempt=EXCLUDED.run_attempt,
		event=EXCLUDED.event,
		status=EXCLUDED.status,
		conclusion=EXCLUDED.conclusion,
		workflow_id=EXCLUDED.workflow_id,
		check_suite_id=EXCLUDED.check_suite_id,
		check_suite_node_id=EXCLUDED.check_suite_node_id,
		url=EXCLUDED.url,
		html_url=EXCLUDED.html_url,
		pull_requests=EXCLUDED.pull_requests,
		created_at=EXCLUDED.created_at,
		updated_at=EXCLUDED.updated_at,
		run_started_at=EXCLUDED.run_started_at,
		jobs_url=EXCLUDED.jobs_url,
		logs_url=EXCLUDED.logs_url,
		check_suite_url=EXCLUDED.check_suite_url,
		artifacts_url=EXCLUDED.artifacts_url,
		cancel_url=EXCLUDED.cancel_url,
		rerun_url=EXCLUDED.rerun_url,
		head_commit=EXCLUDED.head_commit,
		workflow_url=EXCLUDED.workflow_url,
		repository_url=EXCLUDED.repository_url,
		head_repository_url=EXCLUDED.head_repository_url
  RETURNING xmax::text
)
SELECT
    COUNT(*) AS all_rows,
    SUM(CASE WHEN xmax::int = 0 THEN 1 ELSE 0 END) AS ins,
    SUM(CASE WHEN xmax::int > 0 THEN 1 ELSE 0 END) AS upd
FROM t
`

type UpsertWorkflowRunsParams struct {
	RepoID            uuid.UUID
	ID                int64
	Workflowrunnodeid sql.NullString
	Name              sql.NullString
	Headbranch        sql.NullString
	Runnumber         sql.NullInt32
	Runattempt        sql.NullInt32
	Event             sql.NullString
	Status            sql.NullString
	Conclusion        sql.NullString
	Workflowid        sql.NullInt64
	Checksuiteid      sql.NullInt64
	Checksuitenodeid  sql.NullString
	Url               sql.NullString
	Htmlurl           sql.NullString
	Pullrequest       pgtype.JSONB
	Createdat         sql.NullTime
	Updatedat         sql.NullTime
	Runstartedat      sql.NullTime
	Jobsurl           sql.NullString
	Logsurl           sql.NullString
	Checksuiteurl     sql.NullString
	Artifactsurl      sql.NullString
	Cancelurl         sql.NullString
	Rerunurl          sql.NullString
	Headcommit        pgtype.JSONB
	Workflowurl       sql.NullString
	Repositoryurl     sql.NullString
	Headrepositoryurl sql.NullString
}

type UpsertWorkflowRunsRow struct {
	AllRows int64
	Ins     int64
	Upd     int64
}

func (q *Queries) UpsertWorkflowRuns(ctx context.Context, arg UpsertWorkflowRunsParams) error {
	_, err := q.db.Exec(ctx, upsertWorkflowRuns,
		arg.RepoID,
		arg.ID,
		arg.Workflowrunnodeid,
		arg.Name,
		arg.Headbranch,
		arg.Runnumber,
		arg.Runattempt,
		arg.Event,
		arg.Status,
		arg.Conclusion,
		arg.Workflowid,
		arg.Checksuiteid,
		arg.Checksuitenodeid,
		arg.Url,
		arg.Htmlurl,
		arg.Pullrequest,
		arg.Createdat,
		arg.Updatedat,
		arg.Runstartedat,
		arg.Jobsurl,
		arg.Logsurl,
		arg.Checksuiteurl,
		arg.Artifactsurl,
		arg.Cancelurl,
		arg.Rerunurl,
		arg.Headcommit,
		arg.Workflowurl,
		arg.Repositoryurl,
		arg.Headrepositoryurl,
	)
	return err
}

const upsertWorkflowsInPublic = `-- name: UpsertWorkflowsInPublic :exec
WITH t AS (
  INSERT INTO public.github_actions_workflows(
	repo_id, 
	id,
	workflow_node_id,
	name,
	path,
	state,
	created_at,
	updated_at,
	url,
	html_url,
	badge_url
	) 
  VALUES(
    $1::uuid,
	$2::BIGINT,
	$3,
	$4,
	$5,
	$6,
	$7,
	$8,
	$9,
	$10,
	$11) 
  ON CONFLICT (id)
  DO UPDATE
  SET repo_id=EXCLUDED.repo_id,
      id=EXCLUDED.id,
      workflow_node_id=EXCLUDED.workflow_node_id,
      name=EXCLUDED.name,
      path=EXCLUDED.path,
      state=EXCLUDED.state,
      created_at=EXCLUDED.created_at,
      updated_at=EXCLUDED.updated_at,
      url=EXCLUDED.url,
      html_url=EXCLUDED.html_url,
      badge_url=EXCLUDED.badge_url
  RETURNING xmax::text
) 
SELECT
    COUNT(*) AS all_rows,
    SUM(CASE WHEN xmax::int = 0 THEN 1 ELSE 0 END) AS ins,
    SUM(CASE WHEN xmax::int > 0 THEN 1 ELSE 0 END) AS upd
FROM t
`

type UpsertWorkflowsInPublicParams struct {
	Repoid         uuid.UUID
	ID             int64
	Workflownodeid sql.NullString
	Name           sql.NullString
	Path           sql.NullString
	State          sql.NullString
	Createdat      sql.NullTime
	Updatedat      sql.NullTime
	Url            sql.NullString
	Htmlurl        sql.NullString
	Badgeurl       sql.NullString
}

type UpsertWorkflowsInPublicRow struct {
	AllRows int64
	Ins     int64
	Upd     int64
}

func (q *Queries) UpsertWorkflowsInPublic(ctx context.Context, arg UpsertWorkflowsInPublicParams) error {
	_, err := q.db.Exec(ctx, upsertWorkflowsInPublic,
		arg.Repoid,
		arg.ID,
		arg.Workflownodeid,
		arg.Name,
		arg.Path,
		arg.State,
		arg.Createdat,
		arg.Updatedat,
		arg.Url,
		arg.Htmlurl,
		arg.Badgeurl,
	)
	return err
}
