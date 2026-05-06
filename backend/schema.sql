CREATE TABLE IF NOT EXISTS media (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    path TEXT NOT NULL,
    media_type TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    file_extension TEXT NOT NULL,
    poster_path TEXT,
    rating TEXT,
    year INTEGER,
    description TEXT
);

CREATE TABLE IF NOT EXISTS tvshows (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    path TEXT NOT NULL,
    poster_path TEXT,
    rating TEXT,
    year INTEGER,
    description TEXT
);

CREATE TABLE IF NOT EXISTS seasons (
    id SERIAL PRIMARY KEY,
    tvshow_id INTEGER NOT NULL REFERENCES tvshows(id) ON DELETE CASCADE,
    number INTEGER NOT NULL,
    title TEXT NOT NULL,
    path TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS episodes (
    id SERIAL PRIMARY KEY,
    season_id INTEGER NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    number INTEGER NOT NULL,
    title TEXT NOT NULL,
    path TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    rating TEXT
);

-- New tables for transcoder
CREATE TABLE IF NOT EXISTS encode_jobs (
    id SERIAL PRIMARY KEY,
    source_path TEXT NOT NULL,
    destination_path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    preset TEXT NOT NULL DEFAULT 'av1_quicksync',
    total_files INTEGER DEFAULT 0,
    completed_files INTEGER DEFAULT 0,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    error_message TEXT,
    current_file TEXT DEFAULT '',
    progress_percent DOUBLE PRECISION DEFAULT 0,
    fps DOUBLE PRECISION DEFAULT 0,
    speed TEXT DEFAULT '',
    eta_seconds INTEGER DEFAULT 0,
    processed_bytes BIGINT DEFAULT 0,
    processed_frames BIGINT DEFAULT 0,
    total_frames BIGINT DEFAULT 0,
    current_file_percent DOUBLE PRECISION DEFAULT 0,
    cleanup_status TEXT NOT NULL DEFAULT 'not_started',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS encode_results (
    id SERIAL PRIMARY KEY,
    job_id INTEGER NOT NULL REFERENCES encode_jobs(id) ON DELETE CASCADE,
    original_folder_size BIGINT NOT NULL,
    encoded_folder_size BIGINT NOT NULL,
    space_saved BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS encode_file_results (
    id SERIAL PRIMARY KEY,
    result_id INTEGER NOT NULL REFERENCES encode_results(id) ON DELETE CASCADE,
    original_path TEXT NOT NULL,
    encoded_path TEXT NOT NULL,
    original_size BIGINT NOT NULL,
    encoded_size BIGINT NOT NULL,
    space_saved BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT,
    ffmpeg_log_tail TEXT,
    verification_status TEXT NOT NULL DEFAULT 'pending',
    cleanup_status TEXT NOT NULL DEFAULT 'not_started',
    started_at TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS encode_job_events (
    id SERIAL PRIMARY KEY,
    job_id INTEGER NOT NULL REFERENCES encode_jobs(id) ON DELETE CASCADE,
    level TEXT NOT NULL DEFAULT 'info',
    message TEXT NOT NULL,
    file_path TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_encode_jobs_status ON encode_jobs(status);
CREATE INDEX IF NOT EXISTS idx_encode_jobs_source_path ON encode_jobs(source_path);
CREATE INDEX IF NOT EXISTS idx_encode_jobs_created_at ON encode_jobs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_encode_job_events_job_id ON encode_job_events(job_id, created_at DESC);

CREATE TABLE IF NOT EXISTS media_scan_runs (
    id SERIAL PRIMARY KEY,
    status TEXT NOT NULL DEFAULT 'running',
    phase TEXT NOT NULL DEFAULT 'starting',
    root_path TEXT DEFAULT '',
    files_indexed INTEGER NOT NULL DEFAULT 0,
    folders_indexed INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMP DEFAULT NOW(),
    completed_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS media_folders (
    path TEXT PRIMARY KEY,
    parent_path TEXT NOT NULL DEFAULT '',
    root_path TEXT NOT NULL,
    name TEXT NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    file_count INTEGER NOT NULL DEFAULT 0,
    direct_file_count INTEGER NOT NULL DEFAULT 0,
    missing BOOLEAN NOT NULL DEFAULT false,
    last_scan_id INTEGER REFERENCES media_scan_runs(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS media_files (
    path TEXT PRIMARY KEY,
    folder_path TEXT NOT NULL,
    root_path TEXT NOT NULL,
    name TEXT NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    extension TEXT NOT NULL DEFAULT '',
    is_encoded BOOLEAN NOT NULL DEFAULT false,
    codec TEXT NOT NULL DEFAULT '',
    resolution TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    width INTEGER NOT NULL DEFAULT 0,
    height INTEGER NOT NULL DEFAULT 0,
    bit_depth INTEGER NOT NULL DEFAULT 0,
    hdr BOOLEAN NOT NULL DEFAULT false,
    dolby_vision BOOLEAN NOT NULL DEFAULT false,
    duration_seconds DOUBLE PRECISION NOT NULL DEFAULT 0,
    probe_status TEXT NOT NULL DEFAULT 'pending',
    probe_error TEXT NOT NULL DEFAULT '',
    missing BOOLEAN NOT NULL DEFAULT false,
    last_scan_id INTEGER REFERENCES media_scan_runs(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_media_folders_parent_path ON media_folders(parent_path);
CREATE INDEX IF NOT EXISTS idx_media_folders_root_path ON media_folders(root_path);
CREATE INDEX IF NOT EXISTS idx_media_folders_missing ON media_folders(missing);
CREATE INDEX IF NOT EXISTS idx_media_files_folder_path ON media_files(folder_path);
CREATE INDEX IF NOT EXISTS idx_media_files_root_path ON media_files(root_path);
CREATE INDEX IF NOT EXISTS idx_media_files_missing ON media_files(missing);
CREATE INDEX IF NOT EXISTS idx_media_scan_runs_started_at ON media_scan_runs(started_at DESC);

CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT NOW()
);

ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS current_file TEXT DEFAULT '';
ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS progress_percent DOUBLE PRECISION DEFAULT 0;
ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS fps DOUBLE PRECISION DEFAULT 0;
ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS speed TEXT DEFAULT '';
ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS eta_seconds INTEGER DEFAULT 0;
ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS processed_bytes BIGINT DEFAULT 0;
ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS processed_frames BIGINT DEFAULT 0;
ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS total_frames BIGINT DEFAULT 0;
ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS current_file_percent DOUBLE PRECISION DEFAULT 0;
ALTER TABLE encode_jobs ADD COLUMN IF NOT EXISTS cleanup_status TEXT NOT NULL DEFAULT 'not_started';
ALTER TABLE encode_file_results ADD COLUMN IF NOT EXISTS error_message TEXT;
ALTER TABLE encode_file_results ADD COLUMN IF NOT EXISTS ffmpeg_log_tail TEXT;
ALTER TABLE encode_file_results ADD COLUMN IF NOT EXISTS verification_status TEXT NOT NULL DEFAULT 'pending';
ALTER TABLE encode_file_results ADD COLUMN IF NOT EXISTS cleanup_status TEXT NOT NULL DEFAULT 'not_started';
ALTER TABLE encode_file_results ADD COLUMN IF NOT EXISTS started_at TIMESTAMP;
ALTER TABLE encode_file_results ADD COLUMN IF NOT EXISTS completed_at TIMESTAMP;

UPDATE media_scan_runs SET status = 'failed', phase = 'interrupted', error_message = 'Scan was interrupted by server restart', completed_at = NOW() WHERE status = 'running';
