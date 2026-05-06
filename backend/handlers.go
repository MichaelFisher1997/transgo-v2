package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"transgo-v2/backend/models"
)

type sourceProbeFile struct {
	Path            string  `json:"path"`
	Name            string  `json:"name"`
	Size            int64   `json:"size"`
	Codec           string  `json:"codec"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	PixFmt          string  `json:"pix_fmt"`
	BitDepth        int     `json:"bit_depth"`
	HDR             bool    `json:"hdr"`
	DolbyVision     bool    `json:"dolby_vision"`
	DurationSeconds float64 `json:"duration_seconds"`
	Error           string  `json:"error,omitempty"`
}

type sourceProbeSummary struct {
	FileCount   int               `json:"file_count"`
	ProbedCount int               `json:"probed_count"`
	TotalSize   int64             `json:"total_size"`
	Codecs      []string          `json:"codecs"`
	Resolutions []string          `json:"resolutions"`
	MaxWidth    int               `json:"max_width"`
	MaxHeight   int               `json:"max_height"`
	MaxBitDepth int               `json:"max_bit_depth"`
	HDR         bool              `json:"hdr"`
	DolbyVision bool              `json:"dolby_vision"`
	Truncated   bool              `json:"truncated"`
	Files       []sourceProbeFile `json:"files"`
}

type batchJobRequest struct {
	SourcePaths []string `json:"source_paths"`
	Preset      string   `json:"preset"`
}

type batchJobCreated struct {
	ID         int64  `json:"id"`
	SourcePath string `json:"source_path"`
	Status     string `json:"status"`
}

type batchJobFailed struct {
	SourcePath string `json:"source_path"`
	Error      string `json:"error"`
}

type batchJobResponse struct {
	Created []batchJobCreated `json:"created"`
	Skipped []batchJobFailed  `json:"skipped"`
	Failed  []batchJobFailed  `json:"failed"`
}

var preflightCache struct {
	sync.Mutex
	at     time.Time
	preset string
	err    error
}

// APIHandlers holds the repository and encoder dependencies
type APIHandlers struct {
	repo          MediaRepository
	encoder       *Encoder
	indexer       *MediaIndexer
	hub           *EventHub
	folderCacheMu sync.Mutex
	folderCache   []models.FolderInfo
	folderCacheAt time.Time
	folderScan    bool
}

// NewAPIHandlers creates a new APIHandlers instance
func NewAPIHandlers(repo MediaRepository, encoder *Encoder, indexer *MediaIndexer, hub *EventHub) *APIHandlers {
	return &APIHandlers{repo: repo, encoder: encoder, indexer: indexer, hub: hub}
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError writes an error response
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]interface{}{"error": message, "status": status})
}

// GetFoldersHandler lists media folders with sizes
func (h *APIHandlers) GetFoldersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if h.indexer != nil {
		folders, err := h.indexer.GetFolderTree(r.Context())
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to load media index: %v", err))
			return
		}
		status, _ := h.indexer.GetScanStatus(r.Context())
		if !status.InProgress && len(folders) == 0 {
			h.indexer.TriggerScan("empty_index")
			status.InProgress = true
			status.Phase = "starting"
		}
		respondJSON(w, http.StatusOK, models.FoldersResponse{Folders: folders, Scan: status})
		return
	}

	h.refreshFoldersAsync()
	respondJSON(w, http.StatusOK, models.FoldersResponse{Folders: []models.FolderInfo{}, Scan: models.MediaScanStatus{InProgress: true, Phase: "scanning"}})
}

func (h *APIHandlers) cachedFolders() ([]models.FolderInfo, time.Time, bool) {
	h.folderCacheMu.Lock()
	defer h.folderCacheMu.Unlock()
	if h.folderCache == nil {
		return nil, time.Time{}, false
	}
	return h.folderCache, h.folderCacheAt, true
}

func (h *APIHandlers) refreshFoldersAsync() {
	h.folderCacheMu.Lock()
	if h.folderScan {
		h.folderCacheMu.Unlock()
		return
	}
	h.folderScan = true
	h.folderCacheMu.Unlock()

	go h.refreshFolders()
}

func (h *APIHandlers) refreshFolders() {
	started := time.Now()
	deadline := started.Add(20 * time.Second)
	dirs := getMediaDirs()
	var folders []models.FolderInfo

	for _, dir := range dirs {
		info, err := getFolderInfoFast(dir, deadline)
		if err != nil {
			log.Printf("Error reading folder %s: %v", dir, err)
			continue
		}
		folders = append(folders, info)
		if time.Now().After(deadline) {
			break
		}
	}

	h.folderCacheMu.Lock()
	h.folderCache = folders
	h.folderCacheAt = time.Now()
	h.folderScan = false
	h.folderCacheMu.Unlock()
	log.Printf("media folder scan completed in %s", time.Since(started).Round(time.Millisecond))
}

// GetFolderContentsHandler lists contents of a specific folder
func (h *APIHandlers) GetFolderContentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/folders/")
	path, _ = url.PathUnescape(path)

	if h.indexer != nil {
		info, err := h.indexer.GetFolderContents(r.Context(), path)
		if err == nil {
			respondJSON(w, http.StatusOK, info)
			return
		}
	}

	info, err := getFolderInfo(path)
	if err != nil {
		respondError(w, http.StatusNotFound, "Folder not found")
		return
	}

	respondJSON(w, http.StatusOK, info)
}

// DeleteMediaFileHandler deletes one video file inside the configured media dirs.
func (h *APIHandlers) DeleteMediaFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Path == "" {
		respondError(w, http.StatusBadRequest, "Missing file path")
		return
	}
	if err := validateMediaPath(req.Path); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		respondError(w, http.StatusNotFound, "File not found")
		return
	}
	if info.IsDir() {
		respondError(w, http.StatusBadRequest, "Refusing to delete a directory")
		return
	}
	if !isVideoFileExt(strings.ToLower(filepath.Ext(req.Path))) {
		respondError(w, http.StatusBadRequest, "Refusing to delete a non-video file")
		return
	}

	size := info.Size()
	if err := os.Remove(req.Path); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete file: %v", err))
		return
	}
	h.invalidateFolderCache()
	if h.indexer != nil {
		h.indexer.RefreshPath(filepath.Dir(req.Path))
	}
	h.hub.Publish("media.folder.updated", map[string]interface{}{"path": filepath.Dir(req.Path)})

	respondJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "path": req.Path, "size": size})
}

// ProbeSourcesHandler returns ffprobe-backed input details for profile recommendations.
func (h *APIHandlers) ProbeSourcesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if len(req.Paths) == 0 {
		respondError(w, http.StatusBadRequest, "Missing source paths")
		return
	}

	files := []string{}
	seen := map[string]bool{}
	for _, path := range req.Paths {
		if err := validateMediaPath(path); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		info, err := os.Stat(path)
		if err != nil {
			respondError(w, http.StatusNotFound, fmt.Sprintf("Source path not found: %s", path))
			return
		}
		if info.IsDir() {
			scanned, err := scanDirectory(path)
			if err != nil {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to scan source path: %v", err))
				return
			}
			for _, file := range scanned {
				if isTransGoOutput(file) {
					continue
				}
				if !seen[file] {
					seen[file] = true
					files = append(files, file)
				}
			}
			continue
		}
		if !isVideoFileExt(strings.ToLower(filepath.Ext(path))) || isTransGoOutput(path) {
			respondError(w, http.StatusBadRequest, "Source path is not a video file")
			return
		}
		if !seen[path] {
			seen[path] = true
			files = append(files, path)
		}
	}

	const maxProbeFiles = 12
	summary := sourceProbeSummary{FileCount: len(files), Codecs: []string{}, Resolutions: []string{}, Files: []sourceProbeFile{}}
	codecSet := map[string]bool{}
	resolutionSet := map[string]bool{}
	for _, file := range files {
		summary.TotalSize += getFileSize(file)
	}
	for i, file := range files {
		if i >= maxProbeFiles {
			summary.Truncated = true
			break
		}
		probe := probeSourceFile(r.Context(), file)
		summary.ProbedCount++
		summary.Files = append(summary.Files, probe)
		if probe.Codec != "" && !codecSet[probe.Codec] {
			codecSet[probe.Codec] = true
			summary.Codecs = append(summary.Codecs, probe.Codec)
		}
		if probe.Width > summary.MaxWidth {
			summary.MaxWidth = probe.Width
		}
		if probe.Height > summary.MaxHeight {
			summary.MaxHeight = probe.Height
		}
		if probe.BitDepth > summary.MaxBitDepth {
			summary.MaxBitDepth = probe.BitDepth
		}
		if probe.HDR {
			summary.HDR = true
		}
		if probe.DolbyVision {
			summary.DolbyVision = true
		}
		resolution := resolutionLabel(probe.Width, probe.Height)
		if resolution != "" && !resolutionSet[resolution] {
			resolutionSet[resolution] = true
			summary.Resolutions = append(summary.Resolutions, resolution)
		}
	}

	respondJSON(w, http.StatusOK, summary)
}

func probeSourceFile(ctx context.Context, path string) sourceProbeFile {
	probe := sourceProbeFile{Path: path, Name: filepath.Base(path), Size: getFileSize(path)}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "v:0", "-show_streams", "-show_format", "-of", "json", path)
	out, err := cmd.Output()
	if err != nil {
		probe.Error = err.Error()
		return probe
	}
	var data struct {
		Streams []struct {
			CodecName      string `json:"codec_name"`
			Width          int    `json:"width"`
			Height         int    `json:"height"`
			PixFmt         string `json:"pix_fmt"`
			ColorTransfer  string `json:"color_transfer"`
			ColorPrimaries string `json:"color_primaries"`
			ColorSpace     string `json:"color_space"`
			SideDataList   []struct {
				SideDataType string `json:"side_data_type"`
			} `json:"side_data_list"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		probe.Error = err.Error()
		return probe
	}
	if len(data.Streams) == 0 {
		probe.Error = "no video stream found"
		return probe
	}
	stream := data.Streams[0]
	probe.Codec = normalizeCodecName(stream.CodecName)
	probe.Width = stream.Width
	probe.Height = stream.Height
	probe.PixFmt = stream.PixFmt
	probe.BitDepth = pixFmtBitDepth(stream.PixFmt)
	probe.HDR = isHDRStream(stream.ColorTransfer, stream.ColorPrimaries, stream.ColorSpace, stream.SideDataList)
	probe.DolbyVision = hasDolbyVision(stream.SideDataList)
	if duration, err := strconv.ParseFloat(data.Format.Duration, 64); err == nil {
		probe.DurationSeconds = duration
	}
	return probe
}

func normalizeCodecName(codec string) string {
	switch strings.ToLower(codec) {
	case "hevc", "h265":
		return "HEVC"
	case "h264":
		return "AVC"
	case "av1":
		return "AV1"
	case "vc1":
		return "VC-1"
	default:
		return strings.ToUpper(codec)
	}
}

func pixFmtBitDepth(pixFmt string) int {
	if strings.Contains(pixFmt, "12") {
		return 12
	}
	if strings.Contains(pixFmt, "10") || strings.Contains(pixFmt, "p010") {
		return 10
	}
	if pixFmt != "" {
		return 8
	}
	return 0
}

func isHDRStream(transfer, primaries, colorSpace string, sideData []struct {
	SideDataType string `json:"side_data_type"`
}) bool {
	transfer = strings.ToLower(transfer)
	primaries = strings.ToLower(primaries)
	colorSpace = strings.ToLower(colorSpace)
	if strings.Contains(transfer, "smpte2084") || strings.Contains(transfer, "arib-std-b67") || strings.Contains(primaries, "bt2020") || strings.Contains(colorSpace, "bt2020") {
		return true
	}
	for _, item := range sideData {
		name := strings.ToLower(item.SideDataType)
		if strings.Contains(name, "mastering display") || strings.Contains(name, "content light") {
			return true
		}
	}
	return false
}

func hasDolbyVision(sideData []struct {
	SideDataType string `json:"side_data_type"`
}) bool {
	for _, item := range sideData {
		name := strings.ToLower(item.SideDataType)
		if strings.Contains(name, "dovi") || strings.Contains(name, "dolby vision") {
			return true
		}
	}
	return false
}

func resolutionLabel(width, height int) string {
	if width == 0 || height == 0 {
		return ""
	}
	if height >= 2000 || width >= 3800 {
		return "2160p"
	}
	if height >= 1000 {
		return "1080p"
	}
	if height >= 700 {
		return "720p"
	}
	return fmt.Sprintf("%dp", height)
}

func (h *APIHandlers) invalidateFolderCache() {
	h.folderCacheMu.Lock()
	h.folderCache = nil
	h.folderCacheAt = time.Time{}
	h.folderCacheMu.Unlock()
}

// CreateJobHandler creates a new encoding job
func (h *APIHandlers) CreateJobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		SourcePath string `json:"source_path"`
		Preset     string `json:"preset"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.SourcePath == "" {
		respondError(w, http.StatusBadRequest, "source_path is required")
		return
	}
	preset := normalizePreset(req.Preset)

	jobID, err := h.createPendingJob(r.Context(), req.SourcePath, preset)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "Encoder is not ready") {
			status = http.StatusServiceUnavailable
		} else if strings.Contains(err.Error(), "active job") {
			status = http.StatusConflict
		}
		respondError(w, status, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":     jobID,
		"status": "pending",
	})
}

func (h *APIHandlers) CreateJobsBatchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req batchJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if len(req.SourcePaths) == 0 {
		respondError(w, http.StatusBadRequest, "source_paths is required")
		return
	}
	preset := normalizePreset(req.Preset)
	if err := encoderReadyCached(preset); err != nil {
		respondError(w, http.StatusServiceUnavailable, fmt.Sprintf("Encoder is not ready: %v", err))
		return
	}

	resp := batchJobResponse{Created: []batchJobCreated{}, Skipped: []batchJobFailed{}, Failed: []batchJobFailed{}}
	seen := map[string]bool{}
	for _, sourcePath := range req.SourcePaths {
		if sourcePath == "" || seen[sourcePath] {
			continue
		}
		seen[sourcePath] = true
		jobID, err := h.createPendingJob(r.Context(), sourcePath, preset)
		if err != nil {
			entry := batchJobFailed{SourcePath: sourcePath, Error: err.Error()}
			if strings.Contains(err.Error(), "active job") {
				resp.Skipped = append(resp.Skipped, entry)
			} else {
				resp.Failed = append(resp.Failed, entry)
			}
			continue
		}
		resp.Created = append(resp.Created, batchJobCreated{ID: jobID, SourcePath: sourcePath, Status: "pending"})
	}
	respondJSON(w, http.StatusCreated, resp)
}

func normalizePreset(preset string) string {
	if preset != "" {
		return preset
	}
	preset = os.Getenv("ENCODER_PRESET")
	if preset == "" {
		preset = "av1_hardware_fast"
	}
	return preset
}

func (h *APIHandlers) createPendingJob(ctx context.Context, sourcePath, preset string) (int64, error) {
	if err := encoderReadyCached(preset); err != nil {
		return 0, fmt.Errorf("Encoder is not ready: %v", err)
	}
	if err := validateMediaPath(sourcePath); err != nil {
		return 0, err
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("Source path does not exist")
		}
		return 0, err
	}
	writeDir := sourcePath
	if !info.IsDir() {
		writeDir = filepath.Dir(sourcePath)
	}
	if err := ensureWritable(writeDir); err != nil {
		return 0, fmt.Errorf("Source directory is not writable: %v", err)
	}
	active, err := h.repo.HasActiveJobForSource(ctx, sourcePath)
	if err != nil {
		return 0, fmt.Errorf("Failed to check active jobs")
	}
	if active {
		return 0, fmt.Errorf("An active job already exists for this source")
	}

	// Calculate destination path
	suffix := os.Getenv("ENCODER_OUTPUT_SUFFIX")
	if suffix == "" {
		suffix = ".av1"
	}
	dir := filepath.Dir(sourcePath)
	base := filepath.Base(sourcePath)
	destPath := filepath.Join(dir, base+suffix)

	job := &models.EncodeJob{
		SourcePath:      sourcePath,
		DestinationPath: destPath,
		Status:          "pending",
		Preset:          preset,
	}

	jobID, err := h.repo.CreateEncodeJob(ctx, job)
	if err != nil {
		return 0, fmt.Errorf("Failed to create job: %v", err)
	}

	// Submit to encoder queue
	h.encoder.SubmitJob(jobID)
	h.hub.Publish("job.created", map[string]interface{}{"id": jobID, "source_path": sourcePath})
	h.hub.Publish("media.folder.updated", map[string]interface{}{"path": sourcePath})
	return jobID, nil
}

func encoderReady(preset string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return err
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return err
	}
	if _, err := os.Stat("/dev/dri/renderD128"); err != nil {
		return fmt.Errorf("/dev/dri/renderD128 is not available: %w", err)
	}
	encoder := "av1_qsv"
	switch preset {
	case "", "av1_low_cpu", "av1_hardware_fast", "av1_qsv_quality24", "av1_qsv_balanced28", "av1_qsv_smaller30", "av1_qsv", "av1_qsv_quality", "av1_qsv_balanced", "av1_qsv_fast", "av1_quicksync", "av1_vaapi":
		encoder = "av1_qsv"
	case "hevc_vaapi":
		encoder = "hevc_vaapi"
	case "hevc_qsv", "hevc_quicksync":
		encoder = "hevc_qsv"
	default:
		return fmt.Errorf("unsupported hardware preset %q", preset)
	}
	{
		out, err := exec.Command("ffmpeg", "-hide_banner", "-encoders").Output()
		if err != nil {
			return fmt.Errorf("failed to inspect ffmpeg encoders: %w", err)
		}
		if !strings.Contains(string(out), encoder) {
			return fmt.Errorf("ffmpeg does not list %s encoder", encoder)
		}
	}
	return nil
}

func encoderReadyCached(preset string) error {
	preset = normalizePreset(preset)
	preflightCache.Lock()
	if preflightCache.preset == preset && time.Since(preflightCache.at) < 60*time.Second {
		err := preflightCache.err
		preflightCache.Unlock()
		return err
	}
	preflightCache.Unlock()

	err := encoderReady(preset)
	preflightCache.Lock()
	preflightCache.preset = preset
	preflightCache.at = time.Now()
	preflightCache.err = err
	preflightCache.Unlock()
	return err
}

func av1QSVPipelineReady() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-y", "-f", "lavfi", "-i", "nullsrc=s=128x128:d=0.1", "-frames:v", "1", "-c:v", "av1_qsv", "-preset", "faster", "-low_power", "1", "-f", "null", "-")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("AV1 QSV encoder smoke test timed out")
		}
		return fmt.Errorf("AV1 QSV pipeline test failed: %s", strings.TrimSpace(tailString(string(out), 1200)))
	}
	return nil
}

// GetJobsHandler lists all encoding jobs
func (h *APIHandlers) GetJobsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil {
			offset = n
		}
	}

	jobs, err := h.repo.GetEncodeJobs(r.Context(), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get jobs: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, jobs)
}

// GetJobHandler gets a specific job
func (h *APIHandlers) GetJobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	idStr = strings.Split(idStr, "/")[0]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	job, err := h.repo.GetEncodeJob(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}

	respondJSON(w, http.StatusOK, job)
}

// DeleteJobHandler clears a non-running job and removes incomplete temp outputs.
func (h *APIHandlers) DeleteJobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	idStr = strings.Split(idStr, "/")[0]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}
	job, err := h.repo.GetEncodeJob(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}
	if job.Status == "pending" || job.Status == "running" {
		respondError(w, http.StatusBadRequest, "Cancel active jobs before clearing them")
		return
	}
	if job.Status == "cancelling" {
		_ = h.encoder.CancelJob(id)
	}
	terminateTrackedFFmpeg(job)
	deletedTemps, failedTemps := cleanupJobTempOutputs(job)
	if err := h.repo.DeleteEncodeJob(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete job: %v", err))
		return
	}
	h.hub.Publish("job.deleted", map[string]interface{}{"id": id})
	h.hub.Publish("stats.updated", map[string]interface{}{"reason": "job.deleted"})
	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "deleted", "temp_deleted": deletedTemps, "temp_failed": failedTemps})
}

// ClearJobsHandler clears all non-running jobs and removes incomplete temp outputs.
func (h *APIHandlers) ClearJobsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	jobs, err := h.repo.GetEncodeJobs(r.Context(), 10000, 0)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to load jobs: %v", err))
		return
	}
	cleared := 0
	skipped := 0
	tempDeleted := 0
	tempFailed := 0
	for _, job := range jobs {
		if job.Status == "pending" || job.Status == "running" {
			skipped++
			continue
		}
		if job.Status == "cancelling" {
			_ = h.encoder.CancelJob(job.ID)
		}
		terminateTrackedFFmpeg(job)
		deleted, failed := cleanupJobTempOutputs(job)
		tempDeleted += deleted
		tempFailed += failed
		if err := h.repo.DeleteEncodeJob(r.Context(), job.ID); err != nil {
			skipped++
			continue
		}
		cleared++
	}
	h.hub.Publish("jobs.cleared", map[string]interface{}{"cleared": cleared, "skipped": skipped})
	h.hub.Publish("stats.updated", map[string]interface{}{"reason": "jobs.cleared"})

	respondJSON(w, http.StatusOK, map[string]interface{}{"cleared": cleared, "skipped": skipped, "temp_deleted": tempDeleted, "temp_failed": tempFailed})
}

// CancelJobHandler cancels a running job
func (h *APIHandlers) CancelJobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	idStr = strings.Split(idStr, "/")[0]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	_ = h.repo.MarkEncodeJobCancelling(r.Context(), id)
	if err := h.encoder.CancelJob(id); err != nil {
		// Job might not be running, try to mark as cancelled in DB
		if err := h.repo.CancelEncodeJob(r.Context(), id); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to cancel job: %v", err))
			return
		}
	}
	h.hub.Publish("job.updated", map[string]interface{}{"id": id, "status": "cancelling"})
	h.hub.Publish("stats.updated", map[string]interface{}{"reason": "job.cancelled"})

	respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// RetryJobHandler creates a fresh pending job from an interrupted or failed job.
func (h *APIHandlers) RetryJobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	idStr = strings.Split(idStr, "/")[0]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	oldJob, err := h.repo.GetEncodeJob(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}
	if oldJob.Status != "failed" && oldJob.Status != "cancelled" && oldJob.Status != "completed_with_errors" {
		respondError(w, http.StatusBadRequest, "Can only retry failed, cancelled, or completed-with-errors jobs")
		return
	}
	if err := validateMediaPath(oldJob.SourcePath); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	info, err := os.Stat(oldJob.SourcePath)
	if err != nil {
		respondError(w, http.StatusNotFound, "Source path no longer exists")
		return
	}
	if !info.IsDir() {
		respondError(w, http.StatusBadRequest, "Source path is not a directory")
		return
	}
	active, err := h.repo.HasActiveJobForSource(r.Context(), oldJob.SourcePath)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to check existing jobs: %v", err))
		return
	}
	if active {
		respondError(w, http.StatusConflict, "An active job already exists for this source")
		return
	}

	job := &models.EncodeJob{
		SourcePath:      oldJob.SourcePath,
		DestinationPath: oldJob.DestinationPath,
		Status:          "pending",
		Preset:          oldJob.Preset,
	}
	newID, err := h.repo.CreateEncodeJob(r.Context(), job)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create retry job: %v", err))
		return
	}
	_ = h.repo.AddJobEvent(r.Context(), newID, "info", fmt.Sprintf("Retry of job #%d", id), nil)
	h.encoder.SubmitJob(newID)
	h.hub.Publish("job.created", map[string]interface{}{"id": newID, "source_path": oldJob.SourcePath})

	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": newID, "status": "pending", "retried_from": id})
}

// CleanupJobHandler deletes original files after successful encode
func (h *APIHandlers) CleanupJobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	idStr = strings.Split(idStr, "/")[0]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	job, err := h.repo.GetEncodeJob(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}

	if job.Status != "completed" {
		respondError(w, http.StatusBadRequest, "Can only cleanup completed jobs")
		return
	}

	// Get file results to find original files
	result, err := h.repo.GetEncodeResultByJobID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get result")
		return
	}

	fileResults, err := h.repo.GetEncodeFileResultsByResultID(r.Context(), result.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get file results")
		return
	}

	dryRun := r.URL.Query().Get("dry_run") == "true"
	deleted := 0
	failed := 0
	var candidates []string
	for _, fr := range fileResults {
		if fr.Status == "success" && fr.CleanupStatus != "discarded_encoded" {
			if _, err := os.Stat(fr.EncodedPath); err != nil {
				log.Printf("Refusing to delete original %s because encoded file %s is not available: %v", fr.OriginalPath, fr.EncodedPath, err)
				failed++
				continue
			}
			candidates = append(candidates, fr.OriginalPath)
			if dryRun {
				continue
			}
			if err := os.Remove(fr.OriginalPath); err != nil {
				log.Printf("Failed to delete %s: %v", fr.OriginalPath, err)
				failed++
			} else {
				_, _ = h.repo.(*Repository).db.ExecContext(r.Context(), "UPDATE encode_file_results SET cleanup_status = 'deleted' WHERE id = $1", fr.ID)
				deleted++
			}
		}
	}
	if !dryRun && failed == 0 && deleted > 0 {
		_, _ = h.repo.(*Repository).db.ExecContext(r.Context(), "UPDATE encode_jobs SET cleanup_status = 'completed' WHERE id = $1", id)
	}
	if !dryRun && deleted > 0 {
		if h.indexer != nil {
			h.indexer.RefreshPath(job.SourcePath)
		}
		h.hub.Publish("media.folder.updated", map[string]interface{}{"path": job.SourcePath})
		h.hub.Publish("stats.updated", map[string]interface{}{"reason": "cleanup"})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": deleted,
		"failed":  failed,
		"dry_run": dryRun,
		"files":   candidates,
	})
}

// DiscardJobHandler deletes encoded outputs and keeps originals.
func (h *APIHandlers) DiscardJobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	idStr = strings.Split(idStr, "/")[0]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	job, err := h.repo.GetEncodeJob(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}
	if job.Status != "completed" && job.Status != "completed_with_errors" {
		respondError(w, http.StatusBadRequest, "Can only discard completed jobs")
		return
	}
	if job.CleanupStatus == "completed" {
		respondError(w, http.StatusBadRequest, "Cannot discard after originals have been deleted")
		return
	}

	result, err := h.repo.GetEncodeResultByJobID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get result")
		return
	}
	fileResults, err := h.repo.GetEncodeFileResultsByResultID(r.Context(), result.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get file results")
		return
	}

	dryRun := r.URL.Query().Get("dry_run") == "true"
	deleted := 0
	missing := 0
	failed := 0
	var candidates []string
	for _, fr := range fileResults {
		if fr.Status != "success" || fr.CleanupStatus == "deleted" || fr.CleanupStatus == "discarded_encoded" {
			continue
		}
		candidates = append(candidates, fr.EncodedPath)
		if dryRun {
			continue
		}
		if err := os.Remove(fr.EncodedPath); err != nil {
			if os.IsNotExist(err) {
				missing++
				_, _ = h.repo.(*Repository).db.ExecContext(r.Context(), "UPDATE encode_file_results SET cleanup_status = 'discarded_encoded' WHERE id = $1", fr.ID)
				continue
			}
			log.Printf("Failed to delete encoded output %s: %v", fr.EncodedPath, err)
			failed++
		} else {
			_, _ = h.repo.(*Repository).db.ExecContext(r.Context(), "UPDATE encode_file_results SET cleanup_status = 'discarded_encoded' WHERE id = $1", fr.ID)
			deleted++
		}
	}
	if !dryRun && failed == 0 {
		_, _ = h.repo.(*Repository).db.ExecContext(r.Context(), "UPDATE encode_jobs SET cleanup_status = 'discarded' WHERE id = $1", id)
		_ = h.repo.AddJobEvent(r.Context(), id, "info", "Discarded encoded outputs and kept originals", nil)
	}
	if !dryRun && (deleted > 0 || missing > 0) {
		if h.indexer != nil {
			h.indexer.RefreshPath(job.SourcePath)
		}
		h.hub.Publish("media.folder.updated", map[string]interface{}{"path": job.SourcePath})
		h.hub.Publish("stats.updated", map[string]interface{}{"reason": "discard"})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": deleted,
		"missing": missing,
		"failed":  failed,
		"dry_run": dryRun,
		"files":   candidates,
	})
}

// GetResultsHandler lists encode results
func (h *APIHandlers) GetResultsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil {
			offset = n
		}
	}

	results, err := h.repo.GetEncodeResults(r.Context(), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get results: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, results)
}

// GetResultHandler gets a specific result with file details
func (h *APIHandlers) GetResultHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/results/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid result ID")
		return
	}

	result, err := h.repo.GetEncodeResultByJobID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Result not found")
		return
	}

	fileResults, err := h.repo.GetEncodeFileResultsByResultID(r.Context(), result.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get file results")
		return
	}

	events, _ := h.repo.GetJobEvents(r.Context(), id, 200)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"result": result,
		"files":  fileResults,
		"events": events,
	})
}

func (h *APIHandlers) GetJobEventsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	idStr = strings.Split(idStr, "/")[0]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}
	events, err := h.repo.GetJobEvents(r.Context(), id, 200)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get job events")
		return
	}
	respondJSON(w, http.StatusOK, events)
}

func (h *APIHandlers) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *APIHandlers) PreflightHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	checks := map[string]map[string]interface{}{}
	add := func(name string, ok bool, msg string) {
		checks[name] = map[string]interface{}{"ok": ok, "message": msg}
	}

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		add("ffmpeg", false, err.Error())
	} else {
		add("ffmpeg", true, "available")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		add("ffprobe", false, err.Error())
	} else {
		add("ffprobe", true, "available")
	}
	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
	out, err := cmd.Output()
	add("av1_vaapi", err == nil && strings.Contains(string(out), "av1_vaapi"), "AV1 VAAPI encoder detection")
	add("av1_qsv", err == nil && strings.Contains(string(out), "av1_qsv"), "AV1 QSV encoder detection")
	add("hevc_vaapi", err == nil && strings.Contains(string(out), "hevc_vaapi"), "HEVC VAAPI encoder detection")
	if _, err := os.Stat("/dev/dri/renderD128"); err != nil {
		add("renderD128", false, "/dev/dri/renderD128 is not available")
	} else {
		add("renderD128", true, "/dev/dri/renderD128 is available")
	}
	if err := av1QSVPipelineReady(); err != nil {
		add("av1_qsv_pipeline", false, err.Error())
	} else {
		add("av1_qsv_pipeline", true, "AV1 QSV encode pipeline ready")
	}
	for _, dir := range getMediaDirs() {
		if err := ensureWritable(dir); err != nil {
			add("media_writable:"+dir, false, err.Error())
		} else {
			add("media_writable:"+dir, true, "writable")
		}
	}
	respondJSON(w, http.StatusOK, checks)
}

func (h *APIHandlers) SettingsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, h.currentSettings(r.Context()))
	case http.MethodPut:
		var req struct {
			EncoderConcurrency int `json:"encoder_concurrency"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		if req.EncoderConcurrency < 1 || req.EncoderConcurrency > 8 {
			respondError(w, http.StatusBadRequest, "encoder_concurrency must be between 1 and 8")
			return
		}
		if err := h.setSetting(r.Context(), "encoder_concurrency", strconv.Itoa(req.EncoderConcurrency)); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save settings: %v", err))
			return
		}
		h.encoder.SetConcurrency(req.EncoderConcurrency)
		h.hub.Publish("settings.updated", h.currentSettings(r.Context()))
		respondJSON(w, http.StatusOK, h.currentSettings(r.Context()))
	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *APIHandlers) currentSettings(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"encoder_concurrency": h.encoder.Concurrency(),
	}
}

func (h *APIHandlers) setSetting(ctx context.Context, key, value string) error {
	_, err := h.repo.(*Repository).db.ExecContext(ctx, `INSERT INTO app_settings (key, value, updated_at) VALUES ($1, $2, NOW()) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, key, value)
	return err
}

func loadEncoderConcurrency(repo *Repository) int {
	concurrency := 0
	var value string
	if err := repo.db.GetContext(context.Background(), &value, `SELECT value FROM app_settings WHERE key = 'encoder_concurrency'`); err == nil {
		if n, err := strconv.Atoi(value); err == nil {
			concurrency = n
		}
	}
	if concurrency <= 0 {
		concurrency = 2
		if env := os.Getenv("ENCODER_CONCURRENCY"); env != "" {
			if n, err := strconv.Atoi(env); err == nil && n > 0 {
				concurrency = n
			}
		}
	}
	if concurrency < 1 {
		return 1
	}
	if concurrency > 8 {
		return 8
	}
	return concurrency
}

// GetStatsHandler returns dashboard statistics
func (h *APIHandlers) GetStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()
	db := h.repo.(*Repository).db

	var potentialSaved int64
	_ = db.GetContext(ctx, &potentialSaved, "SELECT COALESCE(SUM(space_saved), 0) FROM encode_results")

	var spaceFreed int64
	_ = db.GetContext(ctx, &spaceFreed, "SELECT COALESCE(SUM(original_size), 0) FROM encode_file_results WHERE cleanup_status = 'deleted'")

	var activeJobs int
	_ = db.GetContext(ctx, &activeJobs, "SELECT COUNT(*) FROM encode_jobs WHERE status = 'running'")

	var completedJobs int
	_ = db.GetContext(ctx, &completedJobs, "SELECT COUNT(*) FROM encode_jobs WHERE status = 'completed'")

	var totalJobs int
	_ = db.GetContext(ctx, &totalJobs, "SELECT COUNT(*) FROM encode_jobs WHERE archived = false")

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_space_saved":     potentialSaved,
		"potential_space_saved": potentialSaved,
		"space_freed":           spaceFreed,
		"active_jobs":           activeJobs,
		"completed_jobs":        completedJobs,
		"total_jobs":            totalJobs,
	})
}

// Helper functions

func getMediaDirs() []string {
	dirs := os.Getenv("MEDIA_DIRS")
	if dirs == "" {
		return []string{"./media"}
	}
	return strings.Split(dirs, ",")
}

func getFolderInfo(path string) (models.FolderInfo, error) {
	return getFolderInfoUntil(path, time.Time{})
}

func getFolderInfoFast(path string, deadline time.Time) (models.FolderInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return models.FolderInfo{}, err
	}
	if !info.IsDir() {
		return models.FolderInfo{}, fmt.Errorf("not a directory")
	}

	result := models.FolderInfo{
		Path:  path,
		Name:  filepath.Base(path),
		IsDir: true,
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return result, nil
	}
	for _, entry := range entries {
		if !deadline.IsZero() && time.Now().After(deadline) {
			return result, nil
		}
		entryPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			child := getFolderDirectVideoInfo(entryPath, entry.Name(), deadline)
			result.Size += child.Size
			result.FileCount += child.FileCount
			result.Children = append(result.Children, child)
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if isVideoFileExt(ext) {
			size := getFileSize(entryPath)
			result.Size += size
			result.FileCount++
			result.Children = append(result.Children, models.FolderInfo{Path: entryPath, Name: entry.Name(), Size: size, IsDir: false})
		}
	}
	return result, nil
}

func getFolderDirectVideoInfo(path, name string, deadline time.Time) models.FolderInfo {
	result := models.FolderInfo{Path: path, Name: name, IsDir: true}
	entries, err := os.ReadDir(path)
	if err != nil {
		result.FileCount = 1
		return result
	}
	for _, entry := range entries {
		if !deadline.IsZero() && time.Now().After(deadline) {
			return result
		}
		entryPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if isVideoFileExt(ext) {
			size := getFileSize(entryPath)
			result.Size += size
			result.FileCount++
			result.Children = append(result.Children, models.FolderInfo{Path: entryPath, Name: entry.Name(), Size: size, IsDir: false})
		}
	}
	if result.FileCount == 0 {
		result.FileCount = 1
	}
	return result
}

func getFolderInfoUntil(path string, deadline time.Time) (models.FolderInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return models.FolderInfo{}, err
	}

	if !info.IsDir() {
		return models.FolderInfo{}, fmt.Errorf("not a directory")
	}

	result := models.FolderInfo{
		Path:  path,
		Name:  filepath.Base(path),
		IsDir: true,
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return result, nil // Return what we have
	}

	for _, entry := range entries {
		if !deadline.IsZero() && time.Now().After(deadline) {
			return result, nil
		}
		entryPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			child, _ := getFolderInfoUntil(entryPath, deadline)
			result.Size += child.Size
			result.FileCount += child.FileCount
			result.Children = append(result.Children, child)
		} else {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if isVideoFileExt(ext) {
				size := getFileSize(entryPath)
				result.Size += size
				result.FileCount++
				result.Children = append(result.Children, models.FolderInfo{
					Path:  entryPath,
					Name:  entry.Name(),
					Size:  size,
					IsDir: false,
				})
			}
		}
	}

	return result, nil
}

func validateMediaPath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for _, root := range getMediaDirs() {
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(rootAbs, abs)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return nil
		}
	}
	return fmt.Errorf("path is outside configured media directories")
}

func ensureWritable(dir string) error {
	probe, err := os.CreateTemp(dir, ".transgo-write-test-*")
	if err != nil {
		return err
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}

// Proxy handler for development - serves frontend in production
func serveFrontend(w http.ResponseWriter, r *http.Request) {
	// In production, this would serve from frontend/dist
	// For now, just return a message
	w.Header().Set("Content-Type", "text/html")
	io.WriteString(w, "<!DOCTYPE html><html><body><h1>TransGo API Server</h1><p>Frontend not built. Run npm run build in frontend/</p></body></html>")
}
