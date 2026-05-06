package models

import (
	"database/sql"
)

// Media represents a media file
type Media struct {
	ID            int64          `db:"id" json:"id"`
	Title         string         `db:"title" json:"title"`
	Path          string         `db:"path" json:"path"`
	MediaType     string         `db:"media_type" json:"media_type"`
	FileSize      int64          `db:"file_size" json:"file_size"`
	FileExtension string         `db:"file_extension" json:"file_extension"`
	PosterPath    sql.NullString `db:"poster_path" json:"poster_path,omitempty"`
	Rating        sql.NullString `db:"rating" json:"rating,omitempty"`
	Year          sql.NullInt64  `db:"year" json:"year,omitempty"`
	Description   sql.NullString `db:"description" json:"description,omitempty"`
}

// Media type constants
const (
	MediaTypeMovie  = "movie"
	MediaTypeTVShow = "tvshow"
)

// TVShow represents a TV show
type TVShow struct {
	ID          int64          `db:"id" json:"id"`
	Title       string         `db:"title" json:"title"`
	Path        string         `db:"path" json:"path"`
	PosterPath  sql.NullString `db:"poster_path" json:"poster_path,omitempty"`
	Rating      sql.NullString `db:"rating" json:"rating,omitempty"`
	Year        sql.NullInt64  `db:"year" json:"year,omitempty"`
	Description sql.NullString `db:"description" json:"description,omitempty"`
}

// Season represents a TV show season
type Season struct {
	ID       int64  `db:"id" json:"id"`
	TVShowID int64  `db:"tvshow_id" json:"tvshow_id"`
	Number   int    `db:"number" json:"number"`
	Title    string `db:"title" json:"title"`
	Path     string `db:"path" json:"path"`
}

// Episode represents a TV show episode
type Episode struct {
	ID       int64          `db:"id" json:"id"`
	SeasonID int64          `db:"season_id" json:"season_id"`
	Number   int            `db:"number" json:"number"`
	Title    string         `db:"title" json:"title"`
	Path     string         `db:"path" json:"path"`
	FileSize int64          `db:"file_size" json:"file_size"`
	Rating   sql.NullString `db:"rating" json:"rating,omitempty"`
}

// EncodeJob represents an encoding job
type EncodeJob struct {
	ID                 int64   `db:"id" json:"id"`
	SourcePath         string  `db:"source_path" json:"source_path"`
	DestinationPath    string  `db:"destination_path" json:"destination_path"`
	Status             string  `db:"status" json:"status"`
	Preset             string  `db:"preset" json:"preset"`
	TotalFiles         int     `db:"total_files" json:"total_files"`
	CompletedFiles     int     `db:"completed_files" json:"completed_files"`
	StartedAt          *string `db:"started_at" json:"started_at,omitempty"`
	CompletedAt        *string `db:"completed_at" json:"completed_at,omitempty"`
	ErrorMessage       *string `db:"error_message" json:"error_message,omitempty"`
	CurrentFile        string  `db:"current_file" json:"current_file"`
	ProgressPercent    float64 `db:"progress_percent" json:"progress_percent"`
	FPS                float64 `db:"fps" json:"fps"`
	Speed              string  `db:"speed" json:"speed"`
	ETASeconds         int     `db:"eta_seconds" json:"eta_seconds"`
	ProcessedBytes     int64   `db:"processed_bytes" json:"processed_bytes"`
	ProcessedFrames    int64   `db:"processed_frames" json:"processed_frames"`
	TotalFrames        int64   `db:"total_frames" json:"total_frames"`
	CurrentFilePercent float64 `db:"current_file_percent" json:"current_file_percent"`
	FFmpegPID          int     `db:"ffmpeg_pid" json:"ffmpeg_pid"`
	CleanupStatus      string  `db:"cleanup_status" json:"cleanup_status"`
	Archived           bool    `db:"archived" json:"archived"`
	CreatedAt          string  `db:"created_at" json:"created_at"`
}

// EncodeResult represents the overall result of an encoding job
type EncodeResult struct {
	ID                 int64  `db:"id" json:"id"`
	JobID              int64  `db:"job_id" json:"job_id"`
	OriginalFolderSize int64  `db:"original_folder_size" json:"original_folder_size"`
	EncodedFolderSize  int64  `db:"encoded_folder_size" json:"encoded_folder_size"`
	SpaceSaved         int64  `db:"space_saved" json:"space_saved"`
	CreatedAt          string `db:"created_at" json:"created_at"`
}

// EncodeFileResult represents the result for a single file
type EncodeFileResult struct {
	ID                 int64   `db:"id" json:"id"`
	ResultID           int64   `db:"result_id" json:"result_id"`
	OriginalPath       string  `db:"original_path" json:"original_path"`
	EncodedPath        string  `db:"encoded_path" json:"encoded_path"`
	OriginalSize       int64   `db:"original_size" json:"original_size"`
	EncodedSize        int64   `db:"encoded_size" json:"encoded_size"`
	SpaceSaved         int64   `db:"space_saved" json:"space_saved"`
	Status             string  `db:"status" json:"status"`
	ErrorMessage       *string `db:"error_message" json:"error_message,omitempty"`
	FFmpegLogTail      *string `db:"ffmpeg_log_tail" json:"ffmpeg_log_tail,omitempty"`
	VerificationStatus string  `db:"verification_status" json:"verification_status"`
	CleanupStatus      string  `db:"cleanup_status" json:"cleanup_status"`
	StartedAt          *string `db:"started_at" json:"started_at,omitempty"`
	CompletedAt        *string `db:"completed_at" json:"completed_at,omitempty"`
}

// EncodeJobEvent is an audit log entry for a job.
type EncodeJobEvent struct {
	ID        int64   `db:"id" json:"id"`
	JobID     int64   `db:"job_id" json:"job_id"`
	Level     string  `db:"level" json:"level"`
	Message   string  `db:"message" json:"message"`
	FilePath  *string `db:"file_path" json:"file_path,omitempty"`
	CreatedAt string  `db:"created_at" json:"created_at"`
}

// FolderInfo represents a folder with its size and contents
type FolderInfo struct {
	Path            string       `json:"path" db:"path"`
	Name            string       `json:"name" db:"name"`
	Size            int64        `json:"size" db:"size"`
	FileCount       int          `json:"file_count" db:"file_count"`
	DirectFileCount int          `json:"direct_file_count,omitempty" db:"direct_file_count"`
	IsDir           bool         `json:"is_dir" db:"is_dir"`
	IsEncoded       bool         `json:"is_encoded,omitempty" db:"is_encoded"`
	Codec           string       `json:"codec,omitempty" db:"codec"`
	Resolution      string       `json:"resolution,omitempty" db:"resolution"`
	Source          string       `json:"source,omitempty" db:"source"`
	UpdatedAt       string       `json:"updated_at,omitempty" db:"updated_at"`
	Children        []FolderInfo `json:"children,omitempty"`
}

type MediaScanStatus struct {
	InProgress         bool    `json:"in_progress"`
	Phase              string  `json:"phase"`
	RootPath           string  `json:"root_path,omitempty"`
	StartedAt          string  `json:"started_at,omitempty"`
	LastCompletedAt    string  `json:"last_completed_at,omitempty"`
	LastError          string  `json:"last_error,omitempty"`
	FilesIndexed       int     `json:"files_indexed"`
	FoldersIndexed     int     `json:"folders_indexed"`
	IndexedFileCount   int     `json:"indexed_file_count"`
	IndexedFolderCount int     `json:"indexed_folder_count"`
	Progress           float64 `json:"progress"`
}

type FoldersResponse struct {
	Folders []FolderInfo    `json:"folders"`
	Scan    MediaScanStatus `json:"scan"`
}
