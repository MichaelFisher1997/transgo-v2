package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"transgo-v2/backend/models"
)

// Encoder manages encoding jobs with a worker pool
type Encoder struct {
	repo       MediaRepository
	hub        *EventHub
	indexer    *MediaIndexer
	workQueue  chan int64
	stopCh     chan struct{}
	stopOnce   sync.Once
	workers    int
	maxRunning int
	running    int
	slotCond   *sync.Cond
	activeJobs map[int64]context.CancelFunc
	mu         sync.RWMutex
	wg         sync.WaitGroup
}

// NewEncoder creates a new encoder with the specified concurrency
func NewEncoder(repo MediaRepository, hub *EventHub, indexer *MediaIndexer) *Encoder {
	concurrency := 2
	if env := os.Getenv("ENCODER_CONCURRENCY"); env != "" {
		if n, err := fmt.Sscanf(env, "%d", &concurrency); err != nil || n != 1 {
			concurrency = 2
		}
	}

	workerCount := 8
	if concurrency > workerCount {
		workerCount = concurrency
	}
	e := &Encoder{
		repo:       repo,
		hub:        hub,
		indexer:    indexer,
		workQueue:  make(chan int64, 100),
		stopCh:     make(chan struct{}),
		workers:    workerCount,
		maxRunning: concurrency,
		activeJobs: make(map[int64]context.CancelFunc),
	}
	e.slotCond = sync.NewCond(&e.mu)

	// Start workers
	for i := 0; i < e.workers; i++ {
		go e.worker()
	}

	go e.recoverQueuedJobs()

	return e
}

func (e *Encoder) SetConcurrency(max int) {
	if max < 1 {
		max = 1
	}
	if max > e.workers {
		max = e.workers
	}
	e.mu.Lock()
	e.maxRunning = max
	e.slotCond.Broadcast()
	e.mu.Unlock()
	log.Printf("encoder concurrency set to %d", max)
}

func (e *Encoder) Concurrency() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.maxRunning
}

func (e *Encoder) acquireSlot() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for e.running >= e.maxRunning {
		select {
		case <-e.stopCh:
			return false
		default:
		}
		e.slotCond.Wait()
	}
	e.running++
	return true
}

func (e *Encoder) releaseSlot() {
	e.mu.Lock()
	if e.running > 0 {
		e.running--
	}
	e.slotCond.Broadcast()
	e.mu.Unlock()
}

// Stop gracefully shuts down the encoder
func (e *Encoder) Stop() {
	e.stopOnce.Do(func() {
		close(e.stopCh)

		e.mu.RLock()
		active := make(map[int64]context.CancelFunc, len(e.activeJobs))
		for jobID, cancel := range e.activeJobs {
			active[jobID] = cancel
		}
		e.mu.RUnlock()
		e.slotCond.Broadcast()

		for jobID, cancel := range active {
			_ = e.repo.MarkEncodeJobCancelling(context.Background(), jobID)
			cancel()
		}
		e.wg.Wait()
	})
}

// SubmitJob adds a job to the queue
func (e *Encoder) SubmitJob(jobID int64) {
	select {
	case <-e.stopCh:
		log.Printf("encoder is stopping; job %d remains pending for recovery", jobID)
	case e.workQueue <- jobID:
		e.publishJob(jobID, "job.queued")
	default:
		log.Printf("encoder queue full; job %d remains pending for recovery", jobID)
	}
}

func (e *Encoder) publishJob(jobID int64, eventType string) {
	if e.hub == nil {
		return
	}
	job, err := e.repo.GetEncodeJob(context.Background(), jobID)
	if err != nil {
		e.hub.Publish(eventType, map[string]interface{}{"id": jobID})
		return
	}
	e.hub.Publish(eventType, job)
	e.hub.Publish("stats.updated", map[string]interface{}{"reason": eventType})
}

// CancelJob cancels a running job
func (e *Encoder) CancelJob(jobID int64) error {
	e.mu.RLock()
	cancel, exists := e.activeJobs[jobID]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job %d is not running", jobID)
	}

	cancel()
	return nil
}

func (e *Encoder) recoverQueuedJobs() {
	jobs, err := e.repo.GetQueuedEncodeJobs(context.Background())
	if err != nil {
		log.Printf("failed to recover pending jobs: %v", err)
		return
	}
	for _, job := range jobs {
		e.SubmitJob(job.ID)
	}
	if len(jobs) > 0 {
		log.Printf("recovered %d pending encode jobs", len(jobs))
	}
}

func (e *Encoder) worker() {
	for {
		select {
		case <-e.stopCh:
			return
		case jobID := <-e.workQueue:
			select {
			case <-e.stopCh:
				return
			default:
			}
			if !e.acquireSlot() {
				return
			}
			e.wg.Add(1)
			func() {
				defer e.releaseSlot()
				defer e.wg.Done()
				ctx, cancel := context.WithCancel(context.Background())
				e.mu.Lock()
				e.activeJobs[jobID] = cancel
				e.mu.Unlock()

				err := e.processJob(ctx, jobID)

				e.mu.Lock()
				delete(e.activeJobs, jobID)
				e.mu.Unlock()
				cancel()

				if err != nil {
					log.Printf("Job %d failed: %v", jobID, err)
				}
			}()
		}
	}
}

func (e *Encoder) processJob(ctx context.Context, jobID int64) error {
	job, err := e.repo.GetEncodeJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Update status to running
	now := time.Now()
	_, err = e.repo.(*Repository).db.ExecContext(ctx, "UPDATE encode_jobs SET status = 'running', started_at = $1, error_message = NULL WHERE id = $2 AND status = 'pending'", now, jobID)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}
	e.addEvent(ctx, jobID, "info", "Job started", nil)
	e.publishJob(jobID, "job.updated")

	// Scan source directory for video files
	files, err := scanDirectory(job.SourcePath)
	if err != nil {
		e.failJob(ctx, jobID, fmt.Sprintf("failed to scan directory: %v", err))
		return err
	}
	if len(files) == 0 {
		e.failJob(ctx, jobID, "no video files found")
		return fmt.Errorf("no video files found")
	}

	// Update total files
	_, err = e.repo.(*Repository).db.ExecContext(ctx, "UPDATE encode_jobs SET total_files = $1 WHERE id = $2", len(files), jobID)
	if err != nil {
		return fmt.Errorf("failed to update total files: %w", err)
	}

	// Create result record
	originalSize := calculateTotalSize(files)
	result := &models.EncodeResult{
		JobID:              jobID,
		OriginalFolderSize: originalSize,
	}
	resultID, err := e.repo.CreateEncodeResult(ctx, result)
	if err != nil {
		e.failJob(ctx, jobID, fmt.Sprintf("failed to create result: %v", err))
		return err
	}

	// Create file result records
	suffix := os.Getenv("ENCODER_OUTPUT_SUFFIX")
	if suffix == "" {
		suffix = ".av1"
	}

	fileResultMap := make(map[string]int64) // original path -> file result ID
	for _, file := range files {
		ext := filepath.Ext(file)
		base := strings.TrimSuffix(file, ext)
		encodedPath := base + suffix + ext
		if strings.Contains(base, suffix) || fileExists(encodedPath) {
			continue
		}

		fileResult := &models.EncodeFileResult{
			ResultID:     resultID,
			OriginalPath: file,
			EncodedPath:  encodedPath,
			OriginalSize: getFileSize(file),
			Status:       "pending",
		}
		frID, err := e.repo.CreateEncodeFileResult(ctx, fileResult)
		if err != nil {
			log.Printf("Failed to create file result for %s: %v", file, err)
			continue
		}
		fileResultMap[file] = frID
	}
	if len(fileResultMap) == 0 {
		msg := "all matching video files already have encoded outputs"
		_, _ = e.repo.(*Repository).db.ExecContext(ctx, "UPDATE encode_jobs SET status = 'completed', completed_at = NOW(), error_message = $1 WHERE id = $2", msg, jobID)
		e.addEvent(ctx, jobID, "info", msg, nil)
		e.publishJob(jobID, "job.completed")
		return nil
	}

	// Encode each file
	completedCount := 0
	skippedCount := 0
	encodedTotalSize := int64(0)
	spaceSavedTotal := int64(0)
	lastError := ""

	for _, file := range files {
		select {
		case <-ctx.Done():
			_ = e.repo.CancelEncodeJob(context.Background(), jobID)
			e.addEvent(context.Background(), jobID, "warn", "Job cancelled", nil)
			e.publishJob(jobID, "job.updated")
			return ctx.Err()
		default:
		}

		ext := filepath.Ext(file)
		base := strings.TrimSuffix(file, ext)
		encodedPath := base + suffix + ext
		frID := fileResultMap[file]
		if frID == 0 {
			continue
		}
		if shouldSkipForPreset(ctx, file, job.Preset) {
			_, _ = e.repo.(*Repository).db.ExecContext(ctx, "UPDATE encode_file_results SET status = 'skipped', encoded_size = 0, space_saved = 0, verification_status = 'skipped', completed_at = NOW() WHERE id = $1", frID)
			skippedCount++
			e.addEvent(ctx, jobID, "info", "Skipped file because it already matches the target codec", &file)
			_, _ = e.repo.(*Repository).db.ExecContext(ctx, "UPDATE encode_jobs SET completed_files = $1 WHERE id = $2", completedCount+skippedCount, jobID)
			continue
		}

		// Ensure destination directory exists
		destDir := filepath.Dir(encodedPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			log.Printf("Failed to create destination directory for %s: %v", file, err)
			continue
		}

		_ = e.repo.StartEncodeFileResult(ctx, frID)
		e.addEvent(ctx, jobID, "info", "Encoding file", &file)

		tmpPath := tempOutputPath(encodedPath)
		_ = os.Remove(tmpPath)

		// Run FFmpeg into a temp file, then verify and rename atomically.
		logTail, err := e.runFFmpeg(ctx, jobID, file, tmpPath, job.Preset, completedCount, len(fileResultMap))
		if err != nil {
			_ = os.Remove(tmpPath)
			if ctx.Err() != nil {
				_ = e.repo.CancelEncodeJob(context.Background(), jobID)
				e.addEvent(context.Background(), jobID, "warn", "Job cancelled", &file)
				e.publishJob(jobID, "job.updated")
				return ctx.Err()
			}
			log.Printf("Failed to encode %s: %v", file, err)
			msg := summarizeFFmpegError(err, logTail)
			lastError = msg
			_ = e.repo.FailEncodeFileResult(ctx, frID, msg, logTail)
			e.addEvent(ctx, jobID, "error", msg, &file)
			continue
		}

		if err := verifyEncodedFile(ctx, tmpPath); err != nil {
			_ = os.Remove(tmpPath)
			msg := fmt.Sprintf("verification failed: %v", err)
			lastError = msg
			_ = e.repo.FailEncodeFileResult(ctx, frID, msg, logTail)
			e.addEvent(ctx, jobID, "error", msg, &file)
			continue
		}

		if err := os.Rename(tmpPath, encodedPath); err != nil {
			_ = os.Remove(tmpPath)
			msg := fmt.Sprintf("failed to move temp output into place: %v", err)
			lastError = msg
			_ = e.repo.FailEncodeFileResult(ctx, frID, msg, logTail)
			e.addEvent(ctx, jobID, "error", msg, &file)
			continue
		}

		// Update file result
		encodedSize := getFileSize(encodedPath)
		spaceSaved := getFileSize(file) - encodedSize
		_ = e.repo.FinishEncodeFileResult(ctx, frID, encodedSize, spaceSaved, "passed")

		encodedTotalSize += encodedSize
		spaceSavedTotal += spaceSaved
		completedCount++
		if e.indexer != nil {
			e.indexer.RefreshPath(filepath.Dir(file))
		}
		if e.hub != nil {
			e.hub.Publish("media.folder.updated", map[string]interface{}{"path": filepath.Dir(file)})
		}

		// Update job progress
		_, _ = e.repo.(*Repository).db.ExecContext(ctx,
			"UPDATE encode_jobs SET completed_files = $1 WHERE id = $2",
			completedCount, jobID)
		e.publishJob(jobID, "job.updated")
	}

	// Calculate final result
	_, err = e.repo.(*Repository).db.ExecContext(ctx,
		"UPDATE encode_results SET encoded_folder_size = $1, space_saved = $2 WHERE id = $3",
		encodedTotalSize, spaceSavedTotal, resultID)
	if err != nil {
		log.Printf("Failed to update result: %v", err)
	}

	processedCount := completedCount + skippedCount
	if processedCount == len(fileResultMap) {
		err = e.repo.CompleteEncodeJob(ctx, jobID)
		if err != nil {
			log.Printf("Failed to complete job: %v", err)
		}
		e.addEvent(ctx, jobID, "info", "Job completed", nil)
		e.publishJob(jobID, "job.completed")
	} else {
		msg := fmt.Sprintf("completed with errors: %d/%d files encoded, %d skipped", completedCount, len(fileResultMap), skippedCount)
		if lastError != "" {
			msg += ": " + lastError
		}
		_, _ = e.repo.(*Repository).db.ExecContext(ctx, "UPDATE encode_jobs SET status = 'completed_with_errors', completed_at = NOW(), error_message = $1 WHERE id = $2", msg, jobID)
		e.addEvent(ctx, jobID, "warn", msg, nil)
		e.publishJob(jobID, "job.completed")
	}

	log.Printf("Job %d completed: %d/%d files encoded, %d skipped, saved %d bytes", jobID, completedCount, len(files), skippedCount, spaceSavedTotal)
	return nil
}

func (e *Encoder) failJob(ctx context.Context, jobID int64, msg string) {
	err := e.repo.UpdateEncodeJobStatus(ctx, jobID, "failed", 0, &msg)
	if err != nil {
		log.Printf("Failed to mark job %d as failed: %v", jobID, err)
	}
	e.addEvent(ctx, jobID, "error", msg, nil)
	e.publishJob(jobID, "job.updated")
}

func (e *Encoder) addEvent(ctx context.Context, jobID int64, level, message string, filePath *string) {
	if err := e.repo.AddJobEvent(ctx, jobID, level, message, filePath); err != nil {
		log.Printf("failed to write job event: %v", err)
	}
	if e.hub != nil {
		e.hub.Publish("job.event", map[string]interface{}{"job_id": jobID, "level": level, "message": message, "file_path": filePath})
	}
}

func (e *Encoder) runFFmpeg(ctx context.Context, jobID int64, input, output, preset string, completedFiles, totalFiles int) (string, error) {
	var args []string

	// GPU-only presets. Do not add CPU fallback here.
	if preset == "" || isAV1QSVPreset(preset) || preset == "av1_quicksync" || preset == "av1_vaapi" || preset == "av1_hardware_fast" || preset == "av1_qsv_quality24" || preset == "av1_qsv_balanced28" || preset == "av1_qsv_smaller30" {
		args = av1HardwareArgs(ctx, input, output, preset)
	} else if preset == "hevc_vaapi" {
		args = []string{
			"-vaapi_device", "/dev/dri/renderD128",
			"-y",
			"-hwaccel", "vaapi",
			"-hwaccel_device", "/dev/dri/renderD128",
			"-hwaccel_output_format", "vaapi",
			"-i", input,
			"-vf", "scale_vaapi=format=p010",
			"-progress", "pipe:1", "-nostats",
			"-c:v", "hevc_vaapi",
			"-b:v", "5M",
			"-c:a", "copy",
			"-sn",
			output,
		}
	} else if preset == "hevc_qsv" || preset == "hevc_quicksync" {
		args = []string{
			"-y",
			"-i", input,
			"-c:v", "hevc_qsv",
			"-c:a", "copy",
			"-sn",
			"-preset", "medium",
			"-progress", "pipe:1", "-nostats",
			output,
		}
	} else {
		return "", fmt.Errorf("unsupported hardware preset %q", preset)
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	var stderrBuf bytes.Buffer
	if err := cmd.Start(); err != nil {
		return "", err
	}
	_, _ = e.repo.(*Repository).db.ExecContext(context.Background(), "UPDATE encode_jobs SET ffmpeg_pid = $1 WHERE id = $2", cmd.Process.Pid, jobID)
	defer func() {
		_, _ = e.repo.(*Repository).db.ExecContext(context.Background(), "UPDATE encode_jobs SET ffmpeg_pid = 0 WHERE id = $1", jobID)
	}()
	done := make(chan struct{})
	go func() {
		defer close(done)
		io.Copy(&stderrBuf, stderr)
	}()
	meta := probeProgressMeta(ctx, input)
	e.readProgress(ctx, jobID, input, stdout, meta, completedFiles, totalFiles)
	err = cmd.Wait()
	<-done
	logTail := tailString(stderrBuf.String(), 4000)
	if err != nil {
		return logTail, fmt.Errorf("ffmpeg failed: %w", err)
	}
	return logTail, nil
}

func av1HardwareArgs(ctx context.Context, input, output, preset string) []string {
	if preset == "av1_low_cpu" {
		return []string{
			"-y",
			"-hwaccel", "vaapi",
			"-hwaccel_output_format", "vaapi",
			"-i", input,
			"-map", "0:v:0",
			"-map", "0:a:0?",
			"-vf", "hwmap=derive_device=qsv,format=qsv",
			"-progress", "pipe:1", "-nostats",
			"-c:v", "av1_qsv",
			"-preset", "slow",
			"-c:a", "copy",
			"-sn",
			output,
		}
	}
	codec := probeVideoCodec(ctx, input)
	qualityArgs := av1QualityArgs(preset)
	switch codec {
	case "hevc", "h265":
		args := []string{
			"-y",
			"-hwaccel", "qsv",
			"-hwaccel_output_format", "qsv",
			"-c:v", "hevc_qsv",
			"-i", input,
			"-map", "0:v:0",
			"-map", "0:a:0?",
			"-progress", "pipe:1", "-nostats",
			"-c:v", "av1_qsv",
			"-preset", "faster",
			"-low_power", "1",
			"-async_depth", "16",
		}
		args = append(args, qualityArgs...)
		args = append(args,
			"-c:a", "copy",
			"-sn",
			output,
		)
		return args
	case "h264":
		args := []string{
			"-y",
			"-hwaccel", "qsv",
			"-hwaccel_output_format", "qsv",
			"-c:v", "h264_qsv",
			"-i", input,
			"-map", "0:v:0",
			"-map", "0:a:0?",
			"-progress", "pipe:1", "-nostats",
			"-c:v", "av1_qsv",
			"-preset", "faster",
			"-low_power", "1",
			"-async_depth", "16",
		}
		args = append(args, qualityArgs...)
		args = append(args,
			"-c:a", "copy",
			"-sn",
			output,
		)
		return args
	default:
		return []string{
			"-y",
			"-vaapi_device", "/dev/dri/renderD128",
			"-hwaccel", "vaapi",
			"-hwaccel_device", "/dev/dri/renderD128",
			"-hwaccel_output_format", "vaapi",
			"-i", input,
			"-map", "0:v:0",
			"-map", "0:a:0?",
			"-vf", "scale_vaapi=format=p010",
			"-progress", "pipe:1", "-nostats",
			"-c:v", "av1_vaapi",
			"-c:a", "copy",
			"-sn",
			output,
		}
	}
}

func av1QualityArgs(preset string) []string {
	switch preset {
	case "av1_qsv_quality24":
		return []string{"-global_quality", "24"}
	case "av1_hardware_fast", "av1_qsv_balanced28", "av1_qsv_balanced", "av1_qsv_fast":
		return []string{"-global_quality", "28"}
	case "av1_qsv_smaller30":
		return []string{"-global_quality", "30"}
	default:
		return nil
	}
}

type progressMeta struct {
	durationSeconds float64
	totalFrames     int64
}

func (e *Encoder) readProgress(ctx context.Context, jobID int64, currentFile string, r io.Reader, meta progressMeta, completedFiles, totalFiles int) {
	scanner := bufio.NewScanner(r)
	var fps float64
	var speed string
	var bytesProcessed int64
	var processedFrames int64
	var outSeconds float64
	lastWrite := time.Time{}
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "frame":
			processedFrames, _ = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		case "fps":
			fps, _ = strconv.ParseFloat(parts[1], 64)
		case "speed":
			speed = parts[1]
		case "total_size":
			bytesProcessed, _ = strconv.ParseInt(parts[1], 10, 64)
		case "out_time_ms":
			outMS, _ := strconv.ParseFloat(parts[1], 64)
			outSeconds = outMS / 1000000
		case "out_time_us":
			outUS, _ := strconv.ParseFloat(parts[1], 64)
			outSeconds = outUS / 1000000
		case "progress":
			if time.Since(lastWrite) < time.Second && parts[1] != "end" {
				continue
			}
			lastWrite = time.Now()
			filePercent := calculateFilePercent(meta, outSeconds, processedFrames)
			overallPercent := filePercent
			if totalFiles > 0 {
				overallPercent = ((float64(completedFiles) + filePercent/100) / float64(totalFiles)) * 100
			}
			eta := calculateETA(meta, outSeconds, processedFrames, fps, speed)
			_ = e.repo.UpdateJobProgress(ctx, jobID, currentFile, overallPercent, fps, speed, bytesProcessed, processedFrames, meta.totalFrames, filePercent, eta)
			e.publishJob(jobID, "job.progress")
		}
	}
}

func probeProgressMeta(ctx context.Context, input string) progressMeta {
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=duration,nb_frames,r_frame_rate:stream_tags=NUMBER_OF_FRAMES:format=duration", "-of", "default=noprint_wrappers=1:nokey=0", input)
	out, err := cmd.Output()
	if err != nil {
		return progressMeta{}
	}
	meta := progressMeta{}
	var frameRate float64
	for _, line := range strings.Split(string(out), "\n") {
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "duration":
			if duration, err := strconv.ParseFloat(val, 64); err == nil && duration > 0 {
				meta.durationSeconds = duration
			}
		case "nb_frames":
			if frames, err := strconv.ParseInt(val, 10, 64); err == nil && frames > 0 {
				meta.totalFrames = frames
			}
		case "TAG:NUMBER_OF_FRAMES":
			if frames, err := strconv.ParseInt(val, 10, 64); err == nil && frames > 0 {
				meta.totalFrames = frames
			}
		case "r_frame_rate":
			frameRate = parseFrameRate(val)
		}
	}
	if meta.totalFrames == 0 && meta.durationSeconds > 0 && frameRate > 0 {
		meta.totalFrames = int64(math.Round(meta.durationSeconds * frameRate))
	}
	return meta
}

func parseFrameRate(rate string) float64 {
	parts := strings.Split(rate, "/")
	if len(parts) == 2 {
		num, _ := strconv.ParseFloat(parts[0], 64)
		den, _ := strconv.ParseFloat(parts[1], 64)
		if den != 0 {
			return num / den
		}
	}
	v, _ := strconv.ParseFloat(rate, 64)
	return v
}

func calculateFilePercent(meta progressMeta, outSeconds float64, processedFrames int64) float64 {
	percent := 0.0
	if meta.durationSeconds > 0 && outSeconds > 0 {
		percent = (outSeconds / meta.durationSeconds) * 100
	} else if meta.totalFrames > 0 && processedFrames > 0 {
		percent = (float64(processedFrames) / float64(meta.totalFrames)) * 100
	}
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func calculateETA(meta progressMeta, outSeconds float64, processedFrames int64, fps float64, speed string) int {
	if meta.totalFrames > 0 && processedFrames > 0 && fps > 0 {
		remainingFrames := meta.totalFrames - processedFrames
		if remainingFrames <= 0 {
			return 0
		}
		return int(math.Round(float64(remainingFrames) / fps))
	}
	if meta.durationSeconds <= 0 || outSeconds <= 0 {
		return 0
	}
	remaining := meta.durationSeconds - outSeconds
	if remaining <= 0 {
		return 0
	}
	s := strings.TrimSuffix(strings.TrimSpace(speed), "x")
	multiplier, err := strconv.ParseFloat(s, 64)
	if err == nil && multiplier > 0 {
		return int(math.Round(remaining / multiplier))
	}
	if fps > 0 && meta.totalFrames > 0 {
		return int(math.Round(float64(meta.totalFrames-processedFrames) / fps))
	}
	return 0
}

func scanDirectory(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if isVideoFileExt(ext) {
				files = append(files, path)
			}
		}
		return nil
	})
	return files, err
}

func isVideoFileExt(ext string) bool {
	videoExts := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v"}
	for _, ve := range videoExts {
		if ext == ve {
			return true
		}
	}
	return false
}

func calculateTotalSize(files []string) int64 {
	var total int64
	for _, f := range files {
		total += getFileSize(f)
	}
	return total
}

func getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func tempOutputPath(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return base + ".tmp" + ext
}

func cleanupStaleTempOutputs() {
	suffix := os.Getenv("ENCODER_OUTPUT_SUFFIX")
	if suffix == "" {
		suffix = ".av1"
	}
	marker := suffix + ".tmp"
	deleted := 0
	failed := 0
	for _, root := range getMediaDirs() {
		entries, err := os.ReadDir(root)
		if err != nil {
			log.Printf("stale temp cleanup failed to read %s: %v", root, err)
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			folder := filepath.Join(root, entry.Name())
			files, err := os.ReadDir(folder)
			if err != nil {
				continue
			}
			for _, file := range files {
				if file.IsDir() || !strings.Contains(file.Name(), marker) || !isVideoFileExt(strings.ToLower(filepath.Ext(file.Name()))) {
					continue
				}
				path := filepath.Join(folder, file.Name())
				if err := os.Remove(path); err != nil {
					failed++
					log.Printf("failed to remove stale temp output %s: %v", path, err)
					continue
				}
				deleted++
				log.Printf("removed stale temp output %s", path)
			}
		}
	}
	if deleted > 0 || failed > 0 {
		log.Printf("stale temp cleanup completed: deleted=%d failed=%d", deleted, failed)
	}
}

func cleanupOrphanFFmpegProcesses() {
	suffix := os.Getenv("ENCODER_OUTPUT_SUFFIX")
	if suffix == "" {
		suffix = ".av1"
	}
	marker := suffix + ".tmp"
	entries, err := os.ReadDir("/proc")
	if err != nil {
		log.Printf("orphan ffmpeg cleanup failed to read /proc: %v", err)
		return
	}
	killed := 0
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		cmdlineBytes, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil || len(cmdlineBytes) == 0 {
			continue
		}
		cmdline := strings.ReplaceAll(string(cmdlineBytes), "\x00", " ")
		if !strings.Contains(cmdline, "ffmpeg") || !strings.Contains(cmdline, marker) {
			continue
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		log.Printf("terminating orphan ffmpeg process %d: %s", pid, strings.TrimSpace(cmdline))
		_ = proc.Signal(syscall.SIGTERM)
		killed++
	}
	if killed == 0 {
		return
	}
	time.Sleep(3 * time.Second)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		cmdlineBytes, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil || len(cmdlineBytes) == 0 {
			continue
		}
		cmdline := strings.ReplaceAll(string(cmdlineBytes), "\x00", " ")
		if !strings.Contains(cmdline, "ffmpeg") || !strings.Contains(cmdline, marker) {
			continue
		}
		proc, err := os.FindProcess(pid)
		if err == nil {
			log.Printf("force killing orphan ffmpeg process %d", pid)
			_ = proc.Signal(syscall.SIGKILL)
		}
	}
}

func cleanupJobTempOutputs(job models.EncodeJob) (int, int) {
	deleted := 0
	failed := 0
	seen := map[string]bool{}
	remove := func(path string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		if err := validateMediaPath(path); err != nil {
			failed++
			log.Printf("refusing to remove temp output outside media dirs %s: %v", path, err)
			return
		}
		if err := os.Remove(path); err != nil {
			if !os.IsNotExist(err) {
				failed++
				log.Printf("failed to remove incomplete temp output %s: %v", path, err)
			}
			return
		}
		deleted++
		log.Printf("removed incomplete temp output %s for cleared job %d", path, job.ID)
	}

	if job.CurrentFile != "" {
		remove(tempOutputPath(encodedOutputPath(job.CurrentFile)))
		cleanupDirectTempOutputs(filepath.Dir(job.CurrentFile), seen, remove)
		return deleted, failed
	}
	info, err := os.Stat(job.SourcePath)
	if err != nil {
		return deleted, failed
	}
	if info.IsDir() {
		cleanupDirectTempOutputs(job.SourcePath, seen, remove)
		return deleted, failed
	}
	remove(tempOutputPath(encodedOutputPath(job.SourcePath)))
	return deleted, failed
}

func terminateTrackedFFmpeg(job models.EncodeJob) {
	if job.FFmpegPID <= 0 {
		return
	}
	proc, err := os.FindProcess(job.FFmpegPID)
	if err != nil {
		return
	}
	log.Printf("terminating tracked ffmpeg process %d for job %d", job.FFmpegPID, job.ID)
	_ = proc.Signal(syscall.SIGTERM)
	time.Sleep(2 * time.Second)
	if processStillExists(job.FFmpegPID) {
		log.Printf("force killing tracked ffmpeg process %d for job %d", job.FFmpegPID, job.ID)
		_ = proc.Signal(syscall.SIGKILL)
	}
}

func processStillExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	_, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid)))
	return err == nil
}

func cleanupDirectTempOutputs(dir string, seen map[string]bool, remove func(string)) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	suffix := os.Getenv("ENCODER_OUTPUT_SUFFIX")
	if suffix == "" {
		suffix = ".av1"
	}
	marker := suffix + ".tmp"
	for _, entry := range entries {
		if entry.IsDir() || !strings.Contains(entry.Name(), marker) || !isVideoFileExt(strings.ToLower(filepath.Ext(entry.Name()))) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if seen[path] {
			continue
		}
		remove(path)
	}
}

func encodedOutputPath(path string) string {
	suffix := os.Getenv("ENCODER_OUTPUT_SUFFIX")
	if suffix == "" {
		suffix = ".av1"
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return base + suffix + ext
}

func isTransGoOutput(path string) bool {
	suffix := os.Getenv("ENCODER_OUTPUT_SUFFIX")
	if suffix == "" {
		suffix = ".av1"
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	name := filepath.Base(base)
	return strings.Contains(name, suffix)
}

func tailString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

func summarizeFFmpegError(err error, logTail string) string {
	patterns := []string{
		"Error creating a MFX session",
		"No device available",
		"Error while opening encoder",
		"Could not open encoder before EOF",
		"Nothing was written into output file",
		"Error opening output file",
		"Conversion failed",
	}
	lines := strings.Split(logTail, "\n")
	for _, pattern := range patterns {
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, pattern) {
				return line
			}
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return err.Error()
}

func verifyEncodedFile(ctx context.Context, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return fmt.Errorf("output is empty")
	}
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=codec_type", "-of", "csv=p=0", path)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ffprobe failed: %w", err)
	}
	if !strings.Contains(string(out), "video") {
		return fmt.Errorf("no video stream found")
	}
	return nil
}

func shouldSkipForPreset(ctx context.Context, path, preset string) bool {
	codec := probeVideoCodec(ctx, path)
	switch preset {
	case "", "av1_low_cpu", "av1_hardware_fast", "av1_qsv_quality24", "av1_qsv_balanced28", "av1_qsv_smaller30", "av1_vaapi", "av1_qsv", "av1_qsv_quality", "av1_qsv_balanced", "av1_qsv_fast", "av1_quicksync":
		return codec == "av1"
	case "hevc_vaapi", "hevc_qsv", "hevc_quicksync":
		return codec == "hevc" || codec == "h265"
	default:
		return false
	}
}

func isAV1QSVPreset(preset string) bool {
	switch preset {
	case "av1_low_cpu", "av1_hardware_fast", "av1_qsv_quality24", "av1_qsv_balanced28", "av1_qsv_smaller30", "av1_vaapi", "av1_qsv", "av1_qsv_quality", "av1_qsv_balanced", "av1_qsv_fast":
		return true
	default:
		return false
	}
}

func probeVideoCodec(ctx context.Context, path string) string {
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=codec_name", "-of", "default=noprint_wrappers=1:nokey=1", path)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
