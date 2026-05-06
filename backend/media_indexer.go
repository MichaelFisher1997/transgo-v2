package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"transgo-v2/backend/models"
)

type MediaIndexer struct {
	repo *Repository
	hub  *EventHub
	mu   sync.Mutex
	run  bool
}

type folderAggregate struct {
	path            string
	parentPath      string
	rootPath        string
	name            string
	size            int64
	fileCount       int
	directFileCount int
}

type indexedFile struct {
	path       string
	folderPath string
	rootPath   string
	name       string
	size       int64
	extension  string
	isEncoded  bool
	codec      string
	resolution string
	source     string
}

func NewMediaIndexer(repo *Repository, hub *EventHub) *MediaIndexer {
	return &MediaIndexer{repo: repo, hub: hub}
}

func (m *MediaIndexer) Start() {
	m.TriggerScan("startup")
}

func (m *MediaIndexer) TriggerScan(reason string) {
	m.mu.Lock()
	if m.run {
		m.mu.Unlock()
		return
	}
	m.run = true
	m.mu.Unlock()
	go func() {
		defer func() {
			m.mu.Lock()
			m.run = false
			m.mu.Unlock()
		}()
		m.scan(context.Background(), reason)
	}()
}

func (m *MediaIndexer) RefreshPath(path string) {
	if path == "" {
		m.TriggerScan("refresh")
		return
	}
	m.TriggerScan("path_changed")
}

func (m *MediaIndexer) scan(ctx context.Context, reason string) {
	started := time.Now()
	var scanID int64
	err := m.repo.db.QueryRowxContext(ctx, `INSERT INTO media_scan_runs (status, phase) VALUES ('running', $1) RETURNING id`, reason).Scan(&scanID)
	if err != nil {
		log.Printf("media index scan failed to start: %v", err)
		return
	}
	m.publishScan("media.scan.started", scanID)
	defer func() {
		m.publishScan("media.scan.completed", scanID)
	}()

	totalFiles := 0
	totalFolders := 0
	for _, root := range getMediaDirs() {
		root = filepath.Clean(root)
		select {
		case <-ctx.Done():
			return
		default:
		}
		files, folders, err := collectMediaIndex(root)
		if err != nil {
			log.Printf("media index scan failed for %s: %v", root, err)
			_, _ = m.repo.db.ExecContext(ctx, `UPDATE media_scan_runs SET status = 'failed', phase = 'failed', root_path = $1, error_message = $2, completed_at = NOW() WHERE id = $3`, root, err.Error(), scanID)
			m.hub.Publish("media.scan.failed", map[string]interface{}{"scan_id": scanID, "root_path": root, "error": err.Error()})
			continue
		}
		_, _ = m.repo.db.ExecContext(ctx, `UPDATE media_scan_runs SET phase = 'indexing', root_path = $1 WHERE id = $2`, root, scanID)
		for _, folder := range folders {
			_, err := m.repo.db.ExecContext(ctx, `INSERT INTO media_folders (path, parent_path, root_path, name, size, file_count, direct_file_count, missing, last_scan_id, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, false, $8, NOW())
				ON CONFLICT (path) DO UPDATE SET parent_path = EXCLUDED.parent_path, root_path = EXCLUDED.root_path, name = EXCLUDED.name, size = EXCLUDED.size, file_count = EXCLUDED.file_count, direct_file_count = EXCLUDED.direct_file_count, missing = false, last_scan_id = EXCLUDED.last_scan_id, updated_at = NOW()`,
				folder.path, folder.parentPath, folder.rootPath, folder.name, folder.size, folder.fileCount, folder.directFileCount, scanID)
			if err != nil {
				log.Printf("failed to upsert media folder %s: %v", folder.path, err)
			}
		}
		for _, file := range files {
			_, err := m.repo.db.ExecContext(ctx, `INSERT INTO media_files (path, folder_path, root_path, name, size, extension, is_encoded, codec, resolution, source, missing, last_scan_id, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, false, $11, NOW())
				ON CONFLICT (path) DO UPDATE SET folder_path = EXCLUDED.folder_path, root_path = EXCLUDED.root_path, name = EXCLUDED.name, size = EXCLUDED.size, extension = EXCLUDED.extension, is_encoded = EXCLUDED.is_encoded, codec = EXCLUDED.codec, resolution = EXCLUDED.resolution, source = EXCLUDED.source, missing = false, last_scan_id = EXCLUDED.last_scan_id, updated_at = NOW()`,
				file.path, file.folderPath, file.rootPath, file.name, file.size, file.extension, file.isEncoded, file.codec, file.resolution, file.source, scanID)
			if err != nil {
				log.Printf("failed to upsert media file %s: %v", file.path, err)
			}
		}
		_, _ = m.repo.db.ExecContext(ctx, `UPDATE media_files SET missing = true, updated_at = NOW() WHERE root_path = $1 AND (last_scan_id IS NULL OR last_scan_id <> $2)`, root, scanID)
		_, _ = m.repo.db.ExecContext(ctx, `UPDATE media_folders SET missing = true, updated_at = NOW() WHERE root_path = $1 AND (last_scan_id IS NULL OR last_scan_id <> $2)`, root, scanID)
		totalFiles += len(files)
		totalFolders += len(folders)
		_, _ = m.repo.db.ExecContext(ctx, `UPDATE media_scan_runs SET files_indexed = $1, folders_indexed = $2 WHERE id = $3`, totalFiles, totalFolders, scanID)
		m.hub.Publish("media.scan.progress", map[string]interface{}{"scan_id": scanID, "root_path": root, "files_indexed": totalFiles, "folders_indexed": totalFolders})
	}
	_, _ = m.repo.db.ExecContext(ctx, `UPDATE media_scan_runs SET status = 'completed', phase = 'idle', files_indexed = $1, folders_indexed = $2, completed_at = NOW() WHERE id = $3`, totalFiles, totalFolders, scanID)
	log.Printf("media index scan completed in %s: folders=%d files=%d", time.Since(started).Round(time.Millisecond), totalFolders, totalFiles)
	go m.probeMissingMetadata(context.Background())
}

func (m *MediaIndexer) publishScan(event string, scanID int64) {
	status, err := m.GetScanStatus(context.Background())
	if err != nil {
		m.hub.Publish(event, map[string]interface{}{"scan_id": scanID})
		return
	}
	m.hub.Publish(event, status)
}

func collectMediaIndex(root string) ([]indexedFile, []folderAggregate, error) {
	folders := map[string]*folderAggregate{}
	files := []indexedFile{}
	var ensureFolder func(string) *folderAggregate
	ensureFolder = func(path string) *folderAggregate {
		path = filepath.Clean(path)
		if existing := folders[path]; existing != nil {
			return existing
		}
		parent := filepath.Dir(path)
		if path == root || parent == "." || parent == path {
			parent = ""
		}
		folder := &folderAggregate{path: path, parentPath: parent, rootPath: root, name: filepath.Base(path)}
		folders[path] = folder
		if parent != "" {
			ensureFolder(parent)
		}
		return folder
	}
	ensureFolder(root)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			ensureFolder(path)
			return nil
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !isVideoFileExt(ext) || isTransGoTempOutput(path) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		folderPath := filepath.Dir(path)
		meta := inferMediaMeta(entry.Name())
		file := indexedFile{path: path, folderPath: folderPath, rootPath: root, name: entry.Name(), size: info.Size(), extension: ext, isEncoded: meta.isEncoded, codec: meta.codec, resolution: meta.resolution, source: meta.source}
		files = append(files, file)
		for current := folderPath; current != ""; current = folders[current].parentPath {
			folder := ensureFolder(current)
			folder.size += info.Size()
			folder.fileCount++
			if current == folderPath {
				folder.directFileCount++
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	list := make([]folderAggregate, 0, len(folders))
	for _, folder := range folders {
		list = append(list, *folder)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].path < list[j].path })
	return files, list, nil
}

type inferredMediaMeta struct {
	isEncoded  bool
	codec      string
	resolution string
	source     string
}

func inferMediaMeta(name string) inferredMediaMeta {
	lower := strings.ToLower(name)
	meta := inferredMediaMeta{isEncoded: isTransGoOutput(name)}
	for _, res := range []string{"2160p", "1080p", "720p", "480p"} {
		if strings.Contains(lower, res) {
			meta.resolution = res
			break
		}
	}
	if meta.isEncoded || strings.Contains(lower, "av1") {
		meta.codec = "AV1"
	} else if strings.Contains(lower, "hevc") || strings.Contains(lower, "h265") || strings.Contains(lower, "h.265") || strings.Contains(lower, "x265") {
		meta.codec = "HEVC"
	} else if strings.Contains(lower, "h264") || strings.Contains(lower, "h.264") || strings.Contains(lower, "x264") || strings.Contains(lower, "avc") {
		meta.codec = "AVC"
	}
	if strings.Contains(lower, "remux") || strings.Contains(lower, "blu-ray") || strings.Contains(lower, "bluray") || strings.Contains(lower, "bdremux") {
		meta.source = "Remux"
	} else if strings.Contains(lower, "web-dl") || strings.Contains(lower, "webdl") || strings.Contains(lower, "webrip") {
		meta.source = "WEB-DL"
	} else if strings.Contains(lower, "bdrip") || strings.Contains(lower, "brrip") {
		meta.source = "BDRip"
	}
	return meta
}

func isTransGoTempOutput(path string) bool {
	suffix := os.Getenv("ENCODER_OUTPUT_SUFFIX")
	if suffix == "" {
		suffix = ".av1"
	}
	return strings.Contains(filepath.Base(path), suffix+".tmp")
}

func (m *MediaIndexer) GetScanStatus(ctx context.Context) (models.MediaScanStatus, error) {
	status := models.MediaScanStatus{Phase: "idle"}
	var running struct {
		ID             int64          `db:"id"`
		Phase          string         `db:"phase"`
		RootPath       sql.NullString `db:"root_path"`
		FilesIndexed   int            `db:"files_indexed"`
		FoldersIndexed int            `db:"folders_indexed"`
		StartedAt      string         `db:"started_at"`
	}
	err := m.repo.db.GetContext(ctx, &running, `SELECT id, phase, root_path, files_indexed, folders_indexed, started_at::text FROM media_scan_runs WHERE status = 'running' ORDER BY started_at DESC LIMIT 1`)
	if err == nil {
		status.InProgress = true
		status.Phase = running.Phase
		status.RootPath = running.RootPath.String
		status.FilesIndexed = running.FilesIndexed
		status.FoldersIndexed = running.FoldersIndexed
		status.StartedAt = running.StartedAt
	} else if err != sql.ErrNoRows {
		return status, err
	}
	var completed struct {
		CompletedAt sql.NullString `db:"completed_at"`
		Error       sql.NullString `db:"error_message"`
		Files       int            `db:"files_indexed"`
		Folders     int            `db:"folders_indexed"`
	}
	err = m.repo.db.GetContext(ctx, &completed, `SELECT completed_at::text, error_message, files_indexed, folders_indexed FROM media_scan_runs WHERE status IN ('completed', 'failed') ORDER BY started_at DESC LIMIT 1`)
	if err == nil {
		status.LastCompletedAt = completed.CompletedAt.String
		status.LastError = completed.Error.String
		if !status.InProgress {
			status.FilesIndexed = completed.Files
			status.FoldersIndexed = completed.Folders
		}
	}
	_ = m.repo.db.GetContext(ctx, &status.IndexedFileCount, `SELECT COUNT(*) FROM media_files WHERE missing = false`)
	_ = m.repo.db.GetContext(ctx, &status.IndexedFolderCount, `SELECT COUNT(*) FROM media_folders WHERE missing = false`)
	return status, nil
}

func (m *MediaIndexer) GetFolderTree(ctx context.Context) ([]models.FolderInfo, error) {
	rows := []models.FolderInfo{}
	err := m.repo.db.SelectContext(ctx, &rows, `SELECT path, name, size, file_count, direct_file_count, true AS is_dir, updated_at::text FROM media_folders WHERE missing = false ORDER BY path`)
	if err != nil {
		return nil, err
	}
	byPath := make(map[string]*models.FolderInfo, len(rows))
	for i := range rows {
		rows[i].IsDir = true
		byPath[rows[i].Path] = &rows[i]
	}
	type indexedFileInfo struct {
		models.FolderInfo
		FolderPath string `db:"folder_path"`
	}
	files := []indexedFileInfo{}
	if err := m.repo.db.SelectContext(ctx, &files, `SELECT path, folder_path, name, size, false AS is_dir, is_encoded, codec, resolution, source, updated_at::text FROM media_files WHERE missing = false ORDER BY name`); err != nil {
		return nil, err
	}
	for i := range files {
		file := files[i].FolderInfo
		file.IsDir = false
		if folder := byPath[files[i].FolderPath]; folder != nil {
			folder.Children = append(folder.Children, file)
		}
	}
	rootPtrs := []*models.FolderInfo{}
	for i := range rows {
		parent := filepath.Dir(rows[i].Path)
		if parent == "." || parent == rows[i].Path {
			parent = ""
		}
		if p := byPath[parent]; p != nil {
			p.Children = append(p.Children, rows[i])
		} else {
			rootPtrs = append(rootPtrs, &rows[i])
		}
	}
	roots := make([]models.FolderInfo, 0, len(rootPtrs))
	for _, root := range rootPtrs {
		roots = append(roots, *root)
	}
	return roots, nil
}

func (m *MediaIndexer) GetFolderContents(ctx context.Context, path string) (models.FolderInfo, error) {
	var folder models.FolderInfo
	err := m.repo.db.GetContext(ctx, &folder, `SELECT path, name, size, file_count, direct_file_count, true AS is_dir, updated_at::text FROM media_folders WHERE path = $1 AND missing = false`, path)
	if err != nil {
		return models.FolderInfo{}, err
	}
	folder.IsDir = true
	children := []models.FolderInfo{}
	_ = m.repo.db.SelectContext(ctx, &children, `SELECT path, name, size, file_count, direct_file_count, true AS is_dir, updated_at::text FROM media_folders WHERE parent_path = $1 AND missing = false ORDER BY name`, path)
	files := []models.FolderInfo{}
	_ = m.repo.db.SelectContext(ctx, &files, `SELECT path, name, size, false AS is_dir, is_encoded, codec, resolution, source, updated_at::text FROM media_files WHERE folder_path = $1 AND missing = false ORDER BY name`, path)
	for i := range files {
		files[i].IsDir = false
	}
	folder.Children = append(folder.Children, children...)
	folder.Children = append(folder.Children, files...)
	return folder, nil
}

func (m *MediaIndexer) probeMissingMetadata(ctx context.Context) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return
	}
	type probeRow struct {
		Path string `db:"path"`
		Size int64  `db:"size"`
	}
	rows := []probeRow{}
	if err := m.repo.db.SelectContext(ctx, &rows, `SELECT path, size FROM media_files WHERE missing = false AND probe_status <> 'passed' ORDER BY updated_at DESC LIMIT 10000`); err != nil {
		log.Printf("failed to load media probe queue: %v", err)
		return
	}
	if len(rows) == 0 {
		return
	}
	m.hub.Publish("media.scan.progress", map[string]interface{}{"phase": "probing", "files_indexed": len(rows)})
	for i, row := range rows {
		select {
		case <-ctx.Done():
			return
		default:
		}
		probe := probeSourceFile(ctx, row.Path)
		if probe.Error != "" {
			_, _ = m.repo.db.ExecContext(ctx, `UPDATE media_files SET probe_status = 'failed', probe_error = $1, updated_at = NOW() WHERE path = $2`, probe.Error, row.Path)
			continue
		}
		resolution := resolutionLabel(probe.Width, probe.Height)
		_, _ = m.repo.db.ExecContext(ctx, `UPDATE media_files SET codec = $1, resolution = $2, width = $3, height = $4, bit_depth = $5, hdr = $6, dolby_vision = $7, duration_seconds = $8, probe_status = 'passed', probe_error = '', updated_at = NOW() WHERE path = $9`,
			probe.Codec, resolution, probe.Width, probe.Height, probe.BitDepth, probe.HDR, probe.DolbyVision, probe.DurationSeconds, row.Path)
		if i%10 == 0 || i == len(rows)-1 {
			m.hub.Publish("media.scan.progress", map[string]interface{}{"phase": "probing", "files_indexed": i + 1, "folders_indexed": len(rows)})
		}
	}
	m.hub.Publish("media.scan.completed", map[string]interface{}{"phase": "probing", "files_indexed": len(rows)})
}
