package main

import (
	"context"
	"fmt"
	"os"

	"transgo-v2/backend/models"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// DBTX represents a database transaction
type DBTX interface {
	sqlx.ExtContext
	sqlx.PreparerContext
}

// Repository holds the database connection
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates a new Repository
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Config holds the database configuration
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// NewConfig creates a new Config
func NewConfig() *Config {
	return &Config{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
	}
}

// NewDB creates a new database connection
func NewDB(cfg *Config) (*sqlx.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.DBName)
	if cfg.Password != "" {
		dsn += fmt.Sprintf(" password=%s", cfg.Password)
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := migrateDB(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrateDB(db *sqlx.DB) error {
	statements := []string{
		`ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS current_file TEXT DEFAULT ''`,
		`ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS progress_percent DOUBLE PRECISION DEFAULT 0`,
		`ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS fps DOUBLE PRECISION DEFAULT 0`,
		`ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS speed TEXT DEFAULT ''`,
		`ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS eta_seconds INTEGER DEFAULT 0`,
		`ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS processed_bytes BIGINT DEFAULT 0`,
		`ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS processed_frames BIGINT DEFAULT 0`,
		`ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS total_frames BIGINT DEFAULT 0`,
		`ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS current_file_percent DOUBLE PRECISION DEFAULT 0`,
		`ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS ffmpeg_pid INTEGER DEFAULT 0`,
		`ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS cleanup_status TEXT NOT NULL DEFAULT 'not_started'`,
		`ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS archived BOOLEAN NOT NULL DEFAULT false`,
		`ALTER TABLE encode_file_results ADD COLUMN IF NOT EXISTS error_message TEXT`,
		`ALTER TABLE encode_file_results ADD COLUMN IF NOT EXISTS ffmpeg_log_tail TEXT`,
		`ALTER TABLE encode_file_results ADD COLUMN IF NOT EXISTS verification_status TEXT NOT NULL DEFAULT 'pending'`,
		`ALTER TABLE encode_file_results ADD COLUMN IF NOT EXISTS cleanup_status TEXT NOT NULL DEFAULT 'not_started'`,
		`ALTER TABLE encode_file_results ADD COLUMN IF NOT EXISTS started_at TIMESTAMP`,
		`ALTER TABLE encode_file_results ADD COLUMN IF NOT EXISTS completed_at TIMESTAMP`,
		`CREATE TABLE IF NOT EXISTS encode_job_events (id SERIAL PRIMARY KEY, job_id INTEGER NOT NULL REFERENCES encode_jobs(id) ON DELETE CASCADE, level TEXT NOT NULL DEFAULT 'info', message TEXT NOT NULL, file_path TEXT, created_at TIMESTAMP DEFAULT NOW())`,
		`CREATE INDEX IF NOT EXISTS idx_encode_jobs_status ON encode_jobs(status)`,
		`CREATE INDEX IF NOT EXISTS idx_encode_jobs_source_path ON encode_jobs(source_path)`,
		`CREATE INDEX IF NOT EXISTS idx_encode_jobs_created_at ON encode_jobs(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_encode_job_events_job_id ON encode_job_events(job_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS media_scan_runs (id SERIAL PRIMARY KEY, status TEXT NOT NULL DEFAULT 'running', phase TEXT NOT NULL DEFAULT 'starting', root_path TEXT DEFAULT '', files_indexed INTEGER NOT NULL DEFAULT 0, folders_indexed INTEGER NOT NULL DEFAULT 0, error_message TEXT, started_at TIMESTAMP DEFAULT NOW(), completed_at TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS media_folders (path TEXT PRIMARY KEY, parent_path TEXT NOT NULL DEFAULT '', root_path TEXT NOT NULL, name TEXT NOT NULL, size BIGINT NOT NULL DEFAULT 0, file_count INTEGER NOT NULL DEFAULT 0, direct_file_count INTEGER NOT NULL DEFAULT 0, missing BOOLEAN NOT NULL DEFAULT false, last_scan_id INTEGER REFERENCES media_scan_runs(id) ON DELETE SET NULL, created_at TIMESTAMP DEFAULT NOW(), updated_at TIMESTAMP DEFAULT NOW())`,
		`CREATE TABLE IF NOT EXISTS media_files (path TEXT PRIMARY KEY, folder_path TEXT NOT NULL, root_path TEXT NOT NULL, name TEXT NOT NULL, size BIGINT NOT NULL DEFAULT 0, extension TEXT NOT NULL DEFAULT '', is_encoded BOOLEAN NOT NULL DEFAULT false, codec TEXT NOT NULL DEFAULT '', resolution TEXT NOT NULL DEFAULT '', source TEXT NOT NULL DEFAULT '', width INTEGER NOT NULL DEFAULT 0, height INTEGER NOT NULL DEFAULT 0, bit_depth INTEGER NOT NULL DEFAULT 0, hdr BOOLEAN NOT NULL DEFAULT false, dolby_vision BOOLEAN NOT NULL DEFAULT false, duration_seconds DOUBLE PRECISION NOT NULL DEFAULT 0, probe_status TEXT NOT NULL DEFAULT 'pending', probe_error TEXT NOT NULL DEFAULT '', missing BOOLEAN NOT NULL DEFAULT false, last_scan_id INTEGER REFERENCES media_scan_runs(id) ON DELETE SET NULL, created_at TIMESTAMP DEFAULT NOW(), updated_at TIMESTAMP DEFAULT NOW())`,
		`CREATE INDEX IF NOT EXISTS idx_media_folders_parent_path ON media_folders(parent_path)`,
		`CREATE INDEX IF NOT EXISTS idx_media_folders_root_path ON media_folders(root_path)`,
		`CREATE INDEX IF NOT EXISTS idx_media_folders_missing ON media_folders(missing)`,
		`CREATE INDEX IF NOT EXISTS idx_media_files_folder_path ON media_files(folder_path)`,
		`CREATE INDEX IF NOT EXISTS idx_media_files_root_path ON media_files(root_path)`,
		`CREATE INDEX IF NOT EXISTS idx_media_files_missing ON media_files(missing)`,
		`CREATE INDEX IF NOT EXISTS idx_media_scan_runs_started_at ON media_scan_runs(started_at DESC)`,
		`CREATE TABLE IF NOT EXISTS app_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TIMESTAMP DEFAULT NOW())`,
		`UPDATE media_scan_runs SET status = 'failed', phase = 'interrupted', error_message = 'Scan was interrupted by server restart', completed_at = NOW() WHERE status = 'running'`,
		`UPDATE encode_jobs SET status = 'failed', error_message = 'Job was interrupted by server restart', completed_at = NOW(), ffmpeg_pid = 0 WHERE status IN ('running', 'cancelling')`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}

// GetAllMedia retrieves all media from the database
func (r *Repository) GetAllMedia(ctx context.Context) ([]models.Media, error) {
	var media []models.Media
	err := r.db.SelectContext(ctx, &media, "SELECT * FROM media")
	if err != nil {
		return nil, err
	}
	return media, nil
}

// GetMediaByType retrieves all media of a specific type from the database
func (r *Repository) GetMediaByType(ctx context.Context, mediaType string) ([]models.Media, error) {
	var media []models.Media
	err := r.db.SelectContext(ctx, &media, "SELECT * FROM media WHERE media_type = $1", mediaType)
	if err != nil {
		return nil, err
	}
	return media, nil
}

// GetMediaByPath retrieves a media file by its path
func (r *Repository) GetMediaByPath(ctx context.Context, path string) (models.Media, error) {
	var media models.Media
	err := r.db.GetContext(ctx, &media, "SELECT * FROM media WHERE path = $1", path)
	return media, err
}

// SaveMedia saves a media file to the database
func (r *Repository) SaveMedia(ctx context.Context, media *models.Media) (int64, error) {
	query := `INSERT INTO media (title, path, media_type, file_size, file_extension)
	VALUES ($1, $2, $3, $4, $5) RETURNING id`
	var id int64
	err := r.db.QueryRowxContext(ctx, query, media.Title, media.Path, media.MediaType, media.FileSize, media.FileExtension).Scan(&id)
	return id, err
}

// GetAllTVShows retrieves all TV shows from the database
func (r *Repository) GetAllTVShows(ctx context.Context) ([]models.TVShow, error) {
	var tvshows []models.TVShow
	err := r.db.SelectContext(ctx, &tvshows, "SELECT * FROM tvshows")
	if err != nil {
		return nil, err
	}
	return tvshows, nil
}

// GetTVShowByID retrieves a TV show by its ID
func (r *Repository) GetTVShowByID(ctx context.Context, id int64) (models.TVShow, error) {
	var tvshow models.TVShow
	err := r.db.GetContext(ctx, &tvshow, "SELECT * FROM tvshows WHERE id = $1", id)
	return tvshow, err
}

// GetTVShowByPath retrieves a TV show by its path
func (r *Repository) GetTVShowByPath(ctx context.Context, path string) (models.TVShow, error) {
	var tvshow models.TVShow
	err := r.db.GetContext(ctx, &tvshow, "SELECT * FROM tvshows WHERE path = $1", path)
	return tvshow, err
}

// SaveTVShow saves a TV show to the database
func (r *Repository) SaveTVShow(ctx context.Context, tvshow *models.TVShow) (int64, error) {
	query := `INSERT INTO tvshows (title, path)
	VALUES ($1, $2) RETURNING id`
	var id int64
	err := r.db.QueryRowxContext(ctx, query, tvshow.Title, tvshow.Path).Scan(&id)
	return id, err
}

// GetSeasonsByTVShowID retrieves all seasons for a TV show
func (r *Repository) GetSeasonsByTVShowID(ctx context.Context, tvshowID int64) ([]models.Season, error) {
	var seasons []models.Season
	err := r.db.SelectContext(ctx, &seasons, "SELECT * FROM seasons WHERE tvshow_id = $1 ORDER BY number", tvshowID)
	if err != nil {
		return nil, err
	}
	return seasons, nil
}

// GetSeasonByPath retrieves a season by its path
func (r *Repository) GetSeasonByPath(ctx context.Context, path string) (models.Season, error) {
	var season models.Season
	err := r.db.GetContext(ctx, &season, "SELECT * FROM seasons WHERE path = $1", path)
	return season, err
}

// SaveSeason saves a season to the database
func (r *Repository) SaveSeason(ctx context.Context, season *models.Season) (int64, error) {
	query := `INSERT INTO seasons (tvshow_id, number, title, path)
	VALUES ($1, $2, $3, $4) RETURNING id`
	var id int64
	err := r.db.QueryRowxContext(ctx, query, season.TVShowID, season.Number, season.Title, season.Path).Scan(&id)
	return id, err
}

// GetEpisodesBySeasonID retrieves all episodes for a season
func (r *Repository) GetEpisodesBySeasonID(ctx context.Context, seasonID int64) ([]models.Episode, error) {
	var episodes []models.Episode
	err := r.db.SelectContext(ctx, &episodes, "SELECT * FROM episodes WHERE season_id = $1 ORDER BY number", seasonID)
	if err != nil {
		return nil, err
	}
	return episodes, nil
}

// GetEpisodeByPath retrieves an episode by its path
func (r *Repository) GetEpisodeByPath(ctx context.Context, path string) (models.Episode, error) {
	var episode models.Episode
	err := r.db.GetContext(ctx, &episode, "SELECT * FROM episodes WHERE path = $1", path)
	return episode, err
}

// SaveEpisode saves an episode to the database
func (r *Repository) SaveEpisode(ctx context.Context, episode *models.Episode) (int64, error) {
	query := `INSERT INTO episodes (season_id, number, title, path, file_size)
	VALUES ($1, $2, $3, $4, $5) RETURNING id`
	var id int64
	err := r.db.QueryRowxContext(ctx, query, episode.SeasonID, episode.Number, episode.Title, episode.Path, episode.FileSize).Scan(&id)
	return id, err
}

// CreateEncodeJob creates a new encoding job
func (r *Repository) CreateEncodeJob(ctx context.Context, job *models.EncodeJob) (int64, error) {
	query := `INSERT INTO encode_jobs (source_path, destination_path, status, preset, total_files)
	VALUES ($1, $2, $3, $4, $5) RETURNING id`
	var id int64
	err := r.db.QueryRowxContext(ctx, query, job.SourcePath, job.DestinationPath, job.Status, job.Preset, job.TotalFiles).Scan(&id)
	return id, err
}

// GetEncodeJob retrieves an encoding job by ID
func (r *Repository) GetEncodeJob(ctx context.Context, id int64) (models.EncodeJob, error) {
	var job models.EncodeJob
	err := r.db.GetContext(ctx, &job, "SELECT * FROM encode_jobs WHERE id = $1", id)
	return job, err
}

// GetEncodeJobs retrieves all encoding jobs with pagination
func (r *Repository) GetEncodeJobs(ctx context.Context, limit, offset int) ([]models.EncodeJob, error) {
	jobs := []models.EncodeJob{}
	err := r.db.SelectContext(ctx, &jobs, "SELECT * FROM encode_jobs WHERE archived = false ORDER BY created_at DESC LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *Repository) GetQueuedEncodeJobs(ctx context.Context) ([]models.EncodeJob, error) {
	jobs := []models.EncodeJob{}
	err := r.db.SelectContext(ctx, &jobs, "SELECT * FROM encode_jobs WHERE status = 'pending' AND archived = false ORDER BY created_at ASC")
	return jobs, err
}

func (r *Repository) HasActiveJobForSource(ctx context.Context, sourcePath string) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM encode_jobs WHERE source_path = $1 AND status IN ('pending', 'running', 'cancelling')", sourcePath)
	return count > 0, err
}

// UpdateEncodeJobStatus updates the status of an encoding job
func (r *Repository) UpdateEncodeJobStatus(ctx context.Context, id int64, status string, completedFiles int, errorMsg *string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE encode_jobs SET status = $1, completed_files = $2, error_message = $3 WHERE id = $4", status, completedFiles, errorMsg, id)
	return err
}

// CompleteEncodeJob marks a job as completed
func (r *Repository) CompleteEncodeJob(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE encode_jobs SET status = 'completed', completed_at = NOW() WHERE id = $1", id)
	return err
}

// CancelEncodeJob marks a job as cancelled
func (r *Repository) CancelEncodeJob(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE encode_jobs SET status = 'cancelled', completed_at = NOW() WHERE id = $1", id)
	return err
}

func (r *Repository) MarkEncodeJobCancelling(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE encode_jobs SET status = 'cancelling' WHERE id = $1 AND status IN ('pending', 'running')", id)
	return err
}

func (r *Repository) UpdateJobProgress(ctx context.Context, id int64, currentFile string, progress float64, fps float64, speed string, processedBytes int64, processedFrames int64, totalFrames int64, currentFilePercent float64, etaSeconds int) error {
	_, err := r.db.ExecContext(ctx, "UPDATE encode_jobs SET current_file = $1, progress_percent = $2, fps = $3, speed = $4, processed_bytes = $5, processed_frames = $6, total_frames = $7, current_file_percent = $8, eta_seconds = $9 WHERE id = $10", currentFile, progress, fps, speed, processedBytes, processedFrames, totalFrames, currentFilePercent, etaSeconds, id)
	return err
}

func (r *Repository) AddJobEvent(ctx context.Context, jobID int64, level, message string, filePath *string) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO encode_job_events (job_id, level, message, file_path) VALUES ($1, $2, $3, $4)", jobID, level, message, filePath)
	return err
}

func (r *Repository) GetJobEvents(ctx context.Context, jobID int64, limit int) ([]models.EncodeJobEvent, error) {
	events := []models.EncodeJobEvent{}
	err := r.db.SelectContext(ctx, &events, "SELECT * FROM encode_job_events WHERE job_id = $1 ORDER BY created_at DESC LIMIT $2", jobID, limit)
	return events, err
}

// DeleteEncodeJob archives an encoding job so history data remains available for stats.
func (r *Repository) DeleteEncodeJob(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE encode_jobs SET archived = true WHERE id = $1", id)
	return err
}

// CreateEncodeResult creates a new encode result
func (r *Repository) CreateEncodeResult(ctx context.Context, result *models.EncodeResult) (int64, error) {
	query := `INSERT INTO encode_results (job_id, original_folder_size, encoded_folder_size, space_saved)
	VALUES ($1, $2, $3, $4) RETURNING id`
	var id int64
	err := r.db.QueryRowxContext(ctx, query, result.JobID, result.OriginalFolderSize, result.EncodedFolderSize, result.SpaceSaved).Scan(&id)
	return id, err
}

// GetEncodeResultByJobID retrieves an encode result by job ID
func (r *Repository) GetEncodeResultByJobID(ctx context.Context, jobID int64) (models.EncodeResult, error) {
	var result models.EncodeResult
	err := r.db.GetContext(ctx, &result, "SELECT * FROM encode_results WHERE job_id = $1", jobID)
	return result, err
}

// GetEncodeResults retrieves all encode results with pagination
func (r *Repository) GetEncodeResults(ctx context.Context, limit, offset int) ([]models.EncodeResult, error) {
	results := []models.EncodeResult{}
	err := r.db.SelectContext(ctx, &results, "SELECT * FROM encode_results ORDER BY created_at DESC LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	return results, nil
}

// CreateEncodeFileResult creates a new encode file result
func (r *Repository) CreateEncodeFileResult(ctx context.Context, result *models.EncodeFileResult) (int64, error) {
	query := `INSERT INTO encode_file_results (result_id, original_path, encoded_path, original_size, encoded_size, space_saved, status)
	VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	var id int64
	err := r.db.QueryRowxContext(ctx, query, result.ResultID, result.OriginalPath, result.EncodedPath, result.OriginalSize, result.EncodedSize, result.SpaceSaved, result.Status).Scan(&id)
	return id, err
}

// GetEncodeFileResultsByResultID retrieves all file results for a result
func (r *Repository) GetEncodeFileResultsByResultID(ctx context.Context, resultID int64) ([]models.EncodeFileResult, error) {
	results := []models.EncodeFileResult{}
	err := r.db.SelectContext(ctx, &results, "SELECT * FROM encode_file_results WHERE result_id = $1", resultID)
	if err != nil {
		return nil, err
	}
	return results, nil
}

// UpdateEncodeFileResult updates a file result
func (r *Repository) UpdateEncodeFileResult(ctx context.Context, id int64, encodedSize int64, status string) error {
	spaceSaved := 0 - encodedSize // Will be calculated properly by caller
	_, err := r.db.ExecContext(ctx, "UPDATE encode_file_results SET encoded_size = $1, space_saved = $2, status = $3 WHERE id = $4", encodedSize, spaceSaved, status, id)
	return err
}

func (r *Repository) StartEncodeFileResult(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE encode_file_results SET status = 'running', started_at = NOW() WHERE id = $1", id)
	return err
}

func (r *Repository) FinishEncodeFileResult(ctx context.Context, id int64, encodedSize, spaceSaved int64, verificationStatus string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE encode_file_results SET encoded_size = $1, space_saved = $2, status = 'success', verification_status = $3, completed_at = NOW() WHERE id = $4", encodedSize, spaceSaved, verificationStatus, id)
	return err
}

func (r *Repository) FailEncodeFileResult(ctx context.Context, id int64, message, logTail string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE encode_file_results SET status = 'failed', error_message = $1, ffmpeg_log_tail = $2, verification_status = 'failed', completed_at = NOW() WHERE id = $3", message, logTail, id)
	return err
}
