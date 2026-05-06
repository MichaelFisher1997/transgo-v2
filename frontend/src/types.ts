export interface FolderInfo {
  path: string;
  name: string;
  size: number;
  file_count: number;
  direct_file_count?: number;
  is_dir: boolean;
  is_encoded?: boolean;
  codec?: string;
  resolution?: string;
  source?: string;
  updated_at?: string;
  children?: FolderInfo[];
}

export interface MediaScanStatus {
  in_progress: boolean;
  phase: string;
  root_path?: string;
  started_at?: string;
  last_completed_at?: string;
  last_error?: string;
  files_indexed: number;
  folders_indexed: number;
  indexed_file_count: number;
  indexed_folder_count: number;
  progress: number;
}

export interface FoldersResponse {
  folders: FolderInfo[];
  scan: MediaScanStatus;
}

export interface SourceProbeFile {
  path: string;
  name: string;
  size: number;
  codec: string;
  width: number;
  height: number;
  pix_fmt: string;
  bit_depth: number;
  hdr: boolean;
  dolby_vision: boolean;
  duration_seconds: number;
  error?: string;
}

export interface SourceProbeSummary {
  file_count: number;
  probed_count: number;
  total_size: number;
  codecs: string[];
  resolutions: string[];
  max_width: number;
  max_height: number;
  max_bit_depth: number;
  hdr: boolean;
  dolby_vision: boolean;
  truncated: boolean;
  files: SourceProbeFile[];
}

export interface EncodeJob {
  id: number;
  source_path: string;
  destination_path: string;
  status: string;
  preset: string;
  total_files: number;
  completed_files: number;
  started_at?: string;
  completed_at?: string;
  error_message?: string;
  current_file: string;
  progress_percent: number;
  fps: number;
  speed: string;
  eta_seconds: number;
  processed_bytes: number;
  processed_frames: number;
  total_frames: number;
  current_file_percent: number;
  ffmpeg_pid: number;
  cleanup_status: string;
  created_at: string;
}

export interface EncodeResult {
  id: number;
  job_id: number;
  original_folder_size: number;
  encoded_folder_size: number;
  space_saved: number;
  created_at: string;
}

export interface EncodeFileResult {
  id: number;
  result_id: number;
  original_path: string;
  encoded_path: string;
  original_size: number;
  encoded_size: number;
  space_saved: number;
  status: string;
  error_message?: string;
  ffmpeg_log_tail?: string;
  verification_status: string;
  cleanup_status: string;
  started_at?: string;
  completed_at?: string;
}

export interface EncodeJobEvent {
  id: number;
  job_id: number;
  level: string;
  message: string;
  file_path?: string;
  created_at: string;
}

export interface Stats {
  total_space_saved: number;
  potential_space_saved: number;
  space_freed: number;
  active_jobs: number;
  completed_jobs: number;
  total_jobs: number;
}

export interface AppSettings {
  encoder_concurrency: number;
}
