import { useState, useEffect, useMemo, useCallback } from 'react'
import {
  Folder, FileVideo, Play, ChevronDown, ChevronRight, Search,
  LayoutGrid, List, CheckSquare, Square, Film, HardDrive,
  ArrowUpDown, X, AlertCircle, CheckCircle, Loader2, Trash2
} from 'lucide-react'
import { api } from '../api'
import { EncodeJob, FolderInfo, MediaScanStatus, SourceProbeSummary } from '../types'
import { useServerEvents } from '../useServerEvents'

// ─── Utilities ──────────────────────────────────────────────────

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatDuration(seconds: number): string {
  if (!seconds) return '-'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function summarizeProbe(probe: SourceProbeSummary | null): string {
  if (!probe || probe.probed_count === 0) return 'Inspecting source...'
  const codec = probe.codecs.length === 1 ? probe.codecs[0] : probe.codecs.length > 1 ? 'Mixed codecs' : 'Unknown codec'
  const resolution = probe.resolutions[0] || (probe.max_height ? `${probe.max_height}p` : 'Unknown resolution')
  const flags = [probe.max_bit_depth >= 10 ? `${probe.max_bit_depth}-bit` : '', probe.hdr ? 'HDR' : '', probe.dolby_vision ? 'Dolby Vision' : ''].filter(Boolean)
  return [resolution, codec, ...flags].join(' · ')
}

function estimatedBitrateMbps(probe: SourceProbeSummary | null): number {
  if (!probe || probe.files.length === 0) return 0
  const duration = probe.files.reduce((sum, file) => sum + (file.duration_seconds || 0), 0)
  if (duration <= 0 || probe.total_size <= 0) return 0
  return (probe.total_size * 8) / duration / 1_000_000
}

function profileRecommendation(probe: SourceProbeSummary | null, folderCount = 1): { preset: EncodeProfilePreset; reason: string; warning?: string } {
  if (!probe || probe.probed_count === 0) {
    return { preset: 'av1_qsv_balanced28', reason: 'Using recommended QSV Balanced quality 28 until source details are available.' }
  }
  const codecs = new Set(probe.codecs.map(codec => codec.toUpperCase()))
  const is4K = probe.max_height >= 2000 || probe.max_width >= 3800
  const bitrateMbps = estimatedBitrateMbps(probe)
  const isHighBitrateRemux = bitrateMbps >= 25
  const isBatch = folderCount > 1
  const hasHEVC = codecs.has('HEVC')
  const hasAVC = codecs.has('AVC')
  const hasAV1 = codecs.has('AV1')

  if (hasAV1 && probe.codecs.length === 1) {
    return { preset: 'av1_qsv_balanced28', reason: 'Source is already AV1; the encoder should skip matching files.', warning: 'This source may not need transcoding.' }
  }
  if (is4K && hasHEVC) {
    return { preset: 'av1_qsv_balanced28', reason: '4K HEVC remux-style source detected; QSV Balanced quality 28 is the recommended speed/quality tradeoff.', warning: 'Use Smaller for more savings or Quality for larger/higher-quality output.' }
  }
  if (hasAVC && probe.max_height <= 1200 && (isBatch || isHighBitrateRemux)) {
    return {
      preset: 'av1_qsv_balanced28',
      reason: `${isBatch ? 'Batch encode' : 'High-bitrate AVC remux'} detected; QSV Balanced quality 28 is recommended.`,
    }
  }
  if (hasHEVC || is4K || probe.hdr || probe.max_bit_depth >= 10) {
    return { preset: 'av1_qsv_balanced28', reason: 'High-complexity source detected; QSV Balanced quality 28 is recommended.' }
  }
  if (hasAVC && probe.max_height <= 1200) {
    return { preset: 'av1_qsv_balanced28', reason: '1080p AVC detected; using QSV Balanced quality 28.' }
  }
  return { preset: 'av1_qsv_balanced28', reason: 'Mixed or unknown source details; using QSV Balanced quality 28.' }
}

function extractMeta(name: string) {
  const meta: { resolution?: string; codec?: string; source?: string; isEncoded: boolean } = { isEncoded: false }
  const lower = name.toLowerCase()

  // Already encoded by TransGo output suffix before the real extension.
  meta.isEncoded = /[._-]av1\.[a-z0-9]+$/i.test(name)

  // Resolution
  const resMatch = name.match(/\b(2160p|1080p|720p|480p)\b/i)
  if (resMatch) meta.resolution = resMatch[1]

  // Codec
  if (meta.isEncoded || /\bav1\b/i.test(lower)) meta.codec = 'AV1'
  else if (/\b(hevc|h\.?265|x265)\b/i.test(lower)) meta.codec = 'HEVC'
  else if (/\b(h\.?264|x264|avc)\b/i.test(lower)) meta.codec = 'AVC'

  // Source type
  if (/\b(remux|blu-?ray|bdremux)\b/i.test(lower)) meta.source = 'Remux'
  else if (/\b(web-?dl|webrip)\b/i.test(lower)) meta.source = 'WEB-DL'
  else if (/\b(bdrip|brrip)\b/i.test(lower)) meta.source = 'BDRip'

  return meta
}

function getFolderMeta(folder: FolderInfo) {
  const codecs = new Set<string>()
  const resolutions = new Set<string>()
  const sources = new Set<string>()
  let hasEncoded = false
  let hasNonEncoded = false
  const videoFiles = folder.children?.filter(c => !c.is_dir) || []

  if (videoFiles.length === 0 && (folder.codec || folder.resolution || folder.source || folder.is_encoded !== undefined)) {
    return {
      codecs: folder.codec ? [folder.codec] : [],
      resolutions: folder.resolution ? [folder.resolution] : [],
      sources: folder.source ? [folder.source] : [],
      hasEncoded: Boolean(folder.is_encoded),
      hasNonEncoded: !folder.is_encoded,
      videoCount: folder.file_count,
    }
  }
  
  for (const file of videoFiles) {
    const meta = extractMeta(file.name)
    const codec = file.codec || meta.codec
    const resolution = file.resolution || meta.resolution
    const source = file.source || meta.source
    if (codec) codecs.add(codec)
    if (resolution) resolutions.add(resolution)
    if (source) sources.add(source)
    if (file.is_encoded || meta.isEncoded) hasEncoded = true
    else hasNonEncoded = true
  }
  
  return {
    codecs: Array.from(codecs),
    resolutions: Array.from(resolutions),
    sources: Array.from(sources),
    hasEncoded,
    hasNonEncoded,
    videoCount: videoFiles.length,
  }
}

function isShallowFolder(folder: FolderInfo) {
  return folder.is_dir && !folder.children?.length && folder.file_count > 0 && folder.size === 0
}

const SORT_OPTIONS = [
  { key: 'name', label: 'Name' },
  { key: 'size', label: 'Size' },
  { key: 'fileCount', label: 'Files' },
  { key: 'codec', label: 'Codec' },
] as const

type SortKey = typeof SORT_OPTIONS[number]['key']

type FilterState = {
  codec: string[]
  resolution: string[]
  source: string[]
  encoded: 'all' | 'encoded' | 'not-encoded'
}

type Notice = {
  type: 'info' | 'success' | 'error'
  message: string
}

const FILTER_LABELS: Record<string, string> = {
  'AV1': 'AV1',
  'HEVC': 'HEVC/H.265',
  'AVC': 'AVC/H.264',
  '2160p': '4K (2160p)',
  '1080p': '1080p',
  '720p': '720p',
  'Remux': 'Remux',
  'WEB-DL': 'WEB-DL',
  'BDRip': 'BDRip',
}

const ENCODE_PROFILES = [
  {
    preset: 'av1_low_cpu',
    label: 'AV1 Low CPU',
    description: 'Legacy-style GPU path. VAAPI decode surfaces mapped into AV1 QSV with slow preset; lower CPU per encode, lower FPS.',
  },
  {
    preset: 'av1_qsv_balanced28',
    label: 'AV1 QSV Balanced',
    description: 'Recommended pure QSV path with global_quality 28. Good speed, quality, and size balance.',
  },
  {
    preset: 'av1_qsv_smaller30',
    label: 'AV1 QSV Smaller',
    description: 'Pure QSV with global_quality 30. Smaller files, lower quality, usually faster than higher-quality modes.',
  },
  {
    preset: 'av1_qsv_quality24',
    label: 'AV1 QSV Quality',
    description: 'Pure QSV with global_quality 24. Better quality, larger files, slower encode.',
  },
  {
    preset: 'av1_hardware_fast',
    label: 'AV1 QSV Max Speed',
    description: 'Pure QSV faster preset without explicit global_quality. Fastest, but less predictable size/quality.',
  },
] as const

type EncodeProfilePreset = typeof ENCODE_PROFILES[number]['preset']

type PendingEncode = {
  folders: FolderInfo[]
}

function initialProfileForFolders(folders: FolderInfo[]): EncodeProfilePreset {
  for (const folder of folders) {
    const meta = getFolderMeta(folder)
    const is4K = meta.resolutions.includes('2160p')
    const hasHEVC = meta.codecs.includes('HEVC')
    if (is4K && hasHEVC) return 'av1_qsv_balanced28'
  }
  return 'av1_qsv_balanced28'
}

// ─── Components ─────────────────────────────────────────────────

function MetaBadge({ children, color = 'gray' }: { children: React.ReactNode; color?: 'gray' | 'blue' | 'green' | 'purple' | 'orange' }) {
  const colors = {
    gray: 'bg-gray-700 text-gray-300',
    blue: 'bg-blue-900/50 text-blue-300',
    green: 'bg-green-900/50 text-green-300',
    purple: 'bg-purple-900/50 text-purple-300',
    orange: 'bg-orange-900/50 text-orange-300',
  }
  return (
    <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium ${colors[color]}`}>
      {children}
    </span>
  )
}

function SizeBar({ size, max }: { size: number; max: number }) {
  if (max === 0) return null
  const pct = Math.max(2, (size / max) * 100)
  return (
    <div className="w-24 h-1.5 bg-gray-700 rounded-full overflow-hidden">
      <div className="h-full bg-blue-500 rounded-full" style={{ width: `${pct}%` }} />
    </div>
  )
}

function SizeValue({ folder }: { folder: FolderInfo }) {
  return <span>{formatBytes(folder.size)}</span>
}

function FileCountValue({ folder }: { folder: FolderInfo }) {
  return <span>{folder.file_count}</span>
}

// ─── Tree View ──────────────────────────────────────────────────

function TreeNode({
  folder,
  depth = 0,
  maxSize,
  selected,
  onToggleSelect,
  onEncode,
  searchQuery,
}: {
  folder: FolderInfo
  depth?: number
  maxSize: number
  selected: Set<string>
  onToggleSelect: (path: string) => void
  onEncode: (folder: FolderInfo) => Promise<void>
  searchQuery: string
}) {
  const isUtilityFolder = /^(logs?|samples?|extras?|subs?|subtitles?|\.?[^a-z])/i.test(folder.name)
  const [expanded, setExpanded] = useState(depth < 1 && !isUtilityFolder)
  const [encoding, setEncoding] = useState(false)
  const meta = !folder.is_dir ? extractMeta(folder.name) : null

  const matchesSearch = searchQuery === '' || folder.name.toLowerCase().includes(searchQuery.toLowerCase())
  const hasMatchingChildren = folder.children?.some(c =>
    c.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    c.children?.some(cc => cc.name.toLowerCase().includes(searchQuery.toLowerCase()))
  )

  const showNode = matchesSearch || hasMatchingChildren
  if (!showNode) return null

  const handleEncode = async () => {
    if (folder.file_count === 0) return
    setEncoding(true)
    try {
      await onEncode(folder)
    } finally {
      setEncoding(false)
    }
  }

  const isSelected = selected.has(folder.path)
  const canSelect = folder.is_dir && folder.file_count > 0

  return (
    <div className={depth > 0 ? 'ml-3' : ''}>
      <div
        className={`
          flex items-center gap-2 py-1.5 px-2 rounded group transition-colors
          ${matchesSearch ? 'hover:bg-gray-750' : 'opacity-50'}
        `}
        style={{ paddingLeft: `${depth * 12 + 8}px` }}
      >
        {/* Expand toggle */}
        {folder.is_dir && folder.children && folder.children.length > 0 ? (
          <button
            onClick={() => setExpanded(!expanded)}
            className="text-gray-500 hover:text-gray-300 flex-shrink-0 w-4"
          >
            {expanded ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
          </button>
        ) : (
          <span className="w-4 flex-shrink-0" />
        )}

        {/* Checkbox for batch select */}
        {canSelect && (
          <button
            onClick={() => onToggleSelect(folder.path)}
            className="text-gray-400 hover:text-blue-400 flex-shrink-0"
          >
            {isSelected ? <CheckSquare className="w-4 h-4 text-blue-400" /> : <Square className="w-4 h-4" />}
          </button>
        )}
        {!canSelect && <span className="w-4 flex-shrink-0" />}

        {/* Icon */}
        {folder.is_dir ? (
          <Folder className={`w-4 h-4 flex-shrink-0 ${folder.file_count > 0 ? 'text-yellow-500' : 'text-gray-600'}`} />
        ) : meta?.isEncoded ? (
          <Film className="w-4 h-4 flex-shrink-0 text-green-400" />
        ) : (
          <FileVideo className="w-4 h-4 flex-shrink-0 text-blue-400" />
        )}

        {/* Name */}
        <span className={`flex-1 truncate text-sm ${meta?.isEncoded ? 'text-green-300' : 'text-gray-300'}`}>
          {folder.name}
        </span>

        {/* Meta badges */}
        {meta && (
          <div className="hidden sm:flex items-center gap-1.5 flex-shrink-0">
            {meta.resolution && <MetaBadge color="purple">{meta.resolution}</MetaBadge>}
            {meta.codec && <MetaBadge color="blue">{meta.codec}</MetaBadge>}
            {meta.source && <MetaBadge color="orange">{meta.source}</MetaBadge>}
            {meta.isEncoded && <MetaBadge color="green">AV1</MetaBadge>}
          </div>
        )}

        {/* Size bar + text */}
        <div className="hidden md:flex items-center gap-3 flex-shrink-0 w-48 justify-end">
          {!isShallowFolder(folder) && <SizeBar size={folder.size} max={maxSize} />}
          <span className="text-xs text-gray-400 w-20 text-right tabular-nums">
            <SizeValue folder={folder} />
          </span>
        </div>

        {/* File count */}
        {folder.is_dir && folder.file_count > 0 && (
          <span className="text-xs text-gray-500 w-16 text-right flex-shrink-0 tabular-nums">
            {folder.file_count} file{folder.file_count !== 1 ? 's' : ''}
          </span>
        )}

        {/* Encode button */}
        {folder.is_dir && folder.file_count > 0 && (
          <button
            onClick={handleEncode}
            disabled={encoding}
            className="ml-2 px-2.5 py-1 bg-blue-600/80 hover:bg-blue-500 disabled:bg-gray-700 rounded text-xs flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0"
            title={`Encode ${folder.name}`}
          >
            {encoding ? <Loader2 className="w-3 h-3 animate-spin" /> : <Play className="w-3 h-3" />}
            {encoding ? 'Creating' : 'Encode'}
          </button>
        )}
      </div>

      {/* Children */}
      {expanded && folder.children?.map((child) => (
        <TreeNode
          key={child.path}
          folder={child}
          depth={depth + 1}
          maxSize={maxSize}
          selected={selected}
          onToggleSelect={onToggleSelect}
          onEncode={onEncode}
          searchQuery={searchQuery}
        />
      ))}
    </div>
  )
}

// ─── Grid View ──────────────────────────────────────────────────

function GridCard({
  folder,
  maxSize,
  selected,
  onToggleSelect,
  onEncode,
  onFolderUpdate,
  onNotice,
  activeJob,
}: {
  folder: FolderInfo
  maxSize: number
  selected: Set<string>
  onToggleSelect: (path: string) => void
  onEncode: (folder: FolderInfo) => Promise<void>
  onFolderUpdate: (folder: FolderInfo) => void
  onNotice: (notice: Notice) => void
  activeJob?: EncodeJob
}) {
  const [encoding, setEncoding] = useState(false)
  const [expanded, setExpanded] = useState(false)
  const [loadingFiles, setLoadingFiles] = useState(false)
  const [deletingPath, setDeletingPath] = useState<string | null>(null)
  const [details, setDetails] = useState<FolderInfo | null>(null)
  const activeFolder = details || folder
  const videoChildren = activeFolder.children?.filter(c => !c.is_dir) || []
  const hasEncoded = videoChildren.some(c => extractMeta(c.name).isEncoded)
  const allEncoded = videoChildren.length > 0 && videoChildren.every(c => extractMeta(c.name).isEncoded)
  const hasActiveJob = Boolean(activeJob)

  const handleEncode = async () => {
    if (folder.file_count === 0 || hasActiveJob) return
    setEncoding(true)
    try {
      await onEncode(folder)
    } finally {
      setEncoding(false)
    }
  }

  const isSelected = selected.has(folder.path)

  const loadDetails = async () => {
    setLoadingFiles(true)
    try {
      const next = await api.getFolderContents(folder.path)
      setDetails(next)
      onFolderUpdate(next)
      onNotice({ type: 'success', message: `Loaded ${next.file_count} file${next.file_count === 1 ? '' : 's'} for ${folder.name}.` })
    } catch (err) {
      onNotice({ type: 'error', message: err instanceof Error ? err.message : `Failed to load files for ${folder.name}` })
    } finally {
      setLoadingFiles(false)
    }
  }

  const toggleFiles = async () => {
    const nextExpanded = !expanded
    setExpanded(nextExpanded)
    if (nextExpanded && videoChildren.length === 0) {
      await loadDetails()
    }
  }

  const deleteFile = async (file: FolderInfo) => {
    const ok = window.confirm(`Delete this file?\n\n${file.path}`)
    if (!ok) return
    setDeletingPath(file.path)
    try {
      await api.deleteMediaFile(file.path)
      const source = details || folder
      const nextChildren = (source.children || []).filter(child => child.path !== file.path)
      const nextFolder = {
        ...source,
        children: nextChildren,
        file_count: Math.max(0, source.file_count - 1),
        size: Math.max(0, source.size - file.size),
      }
      setDetails(nextFolder)
      onFolderUpdate(nextFolder)
      onNotice({ type: 'success', message: `Deleted ${file.name}.` })
    } catch (err) {
      onNotice({ type: 'error', message: err instanceof Error ? err.message : `Failed to delete ${file.name}` })
    } finally {
      setDeletingPath(null)
    }
  }

  // Extract year from folder name if present
  const yearMatch = folder.name.match(/\((\d{4})\)/)
  const year = yearMatch ? yearMatch[1] : null
  const cleanName = folder.name.replace(/\s*\(\d{4}\)\s*$/, '').trim()

  return (
    <div className={`bg-gray-800 rounded-lg border transition-all ${
      hasActiveJob ? 'border-blue-400 shadow-[0_0_0_1px_rgba(96,165,250,0.45),0_0_24px_rgba(37,99,235,0.18)]' : allEncoded ? 'border-green-800/50 hover:border-gray-600' : 'border-gray-700 hover:border-gray-600'
    }`}>
      <div className="p-4">
        {/* Header with checkbox */}
        <div className="flex items-start justify-between mb-3">
          <button
            onClick={() => onToggleSelect(folder.path)}
            className="text-gray-400 hover:text-blue-400"
          >
            {isSelected ? <CheckSquare className="w-5 h-5 text-blue-400" /> : <Square className="w-5 h-5" />}
          </button>
          <div className="flex flex-wrap justify-end gap-1.5">
            {activeJob && <MetaBadge color="blue">Job in progress</MetaBadge>}
            {allEncoded && <MetaBadge color="green">All AV1</MetaBadge>}
            {hasEncoded && !allEncoded && <MetaBadge color="green">Partial</MetaBadge>}
          </div>
        </div>

        {/* Title */}
        <h3 className="font-semibold text-gray-200 truncate" title={folder.name}>
          {cleanName}
        </h3>
        {year && <p className="text-sm text-gray-500">{year}</p>}

        {/* Stats */}
        {isShallowFolder(activeFolder) ? (
          <div className="mt-3 rounded border border-gray-700 bg-gray-900/40 px-3 py-2 text-sm text-gray-400">
            Files available. Open to inspect.
          </div>
        ) : (
          <div className="mt-3 space-y-2">
            <div className="flex items-center justify-between text-sm">
              <span className="text-gray-500">Size</span>
              <span className="text-gray-300 tabular-nums"><SizeValue folder={activeFolder} /></span>
            </div>
            <SizeBar size={activeFolder.size} max={maxSize} />
            <div className="flex items-center justify-between text-sm">
              <span className="text-gray-500">Files</span>
              <span className="text-gray-300"><FileCountValue folder={activeFolder} /></span>
            </div>
          </div>
        )}

        {/* Resolution badges from children */}
        <div className="mt-3 flex flex-wrap gap-1">
          {videoChildren.slice(0, 3).map(child => {
            const meta = extractMeta(child.name)
            return (
              <div key={child.path} className="flex items-center gap-1">
                {meta.resolution && <MetaBadge color="purple">{meta.resolution}</MetaBadge>}
                {meta.codec && <MetaBadge color="blue">{meta.codec}</MetaBadge>}
              </div>
            )
          })}
          {videoChildren.length > 3 && (
            <MetaBadge>+{videoChildren.length - 3}</MetaBadge>
          )}
        </div>

        {/* Actions */}
        <div className="mt-4 grid grid-cols-2 gap-2">
          <button
            onClick={toggleFiles}
            disabled={loadingFiles}
            className="px-3 py-2 bg-gray-700 hover:bg-gray-600 disabled:bg-gray-700 disabled:text-gray-500 rounded text-sm flex items-center justify-center gap-2 transition-colors"
          >
            {loadingFiles ? <Loader2 className="w-4 h-4 animate-spin" /> : expanded ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
            Files
          </button>
          <button
            onClick={handleEncode}
            disabled={encoding || allEncoded || hasActiveJob}
            className="px-3 py-2 bg-blue-600 hover:bg-blue-500 disabled:bg-gray-700 disabled:text-gray-500 rounded text-sm flex items-center justify-center gap-2 transition-colors"
          >
            {encoding || hasActiveJob ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
            {encoding ? 'Creating...' : hasActiveJob ? 'In Progress' : allEncoded ? 'Encoded' : 'Encode'}
          </button>
        </div>

        {expanded && (
          <div className="mt-3 border-t border-gray-700 pt-3 space-y-2">
            {videoChildren.length === 0 && !loadingFiles && (
              <div className="text-sm text-gray-500">No direct video files found.</div>
            )}
            {videoChildren.map(file => {
              const meta = extractMeta(file.name)
              const isDeleting = deletingPath === file.path
              return (
                <div key={file.path} className="rounded border border-gray-700 bg-gray-900/40 px-2 py-2">
                  <div className="flex items-start gap-2">
                    <FileVideo className={`mt-0.5 w-4 h-4 flex-shrink-0 ${meta.isEncoded ? 'text-green-400' : 'text-blue-400'}`} />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm text-gray-200" title={file.name}>{file.name}</div>
                      <div className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-gray-500">
                        <span>{formatBytes(file.size)}</span>
                        {meta.resolution && <MetaBadge color="purple">{meta.resolution}</MetaBadge>}
                        {meta.codec && <MetaBadge color="blue">{meta.codec}</MetaBadge>}
                        {meta.source && <MetaBadge color="orange">{meta.source}</MetaBadge>}
                        {meta.isEncoded && <MetaBadge color="green">Transcode</MetaBadge>}
                      </div>
                    </div>
                    <button
                      onClick={() => deleteFile(file)}
                      disabled={isDeleting}
                      className="rounded p-1.5 text-red-300 hover:bg-red-950/60 hover:text-red-100 disabled:text-gray-500"
                      title={`Delete ${file.name}`}
                    >
                      {isDeleting ? <Loader2 className="w-4 h-4 animate-spin" /> : <Trash2 className="w-4 h-4" />}
                    </button>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}

// ─── Filter Button Component ────────────────────────────────────

function FilterButton({
  label,
  options,
  selected,
  onChange,
}: {
  label: string
  options: string[]
  selected: string[]
  onChange: (values: string[]) => void
}) {
  const [open, setOpen] = useState(false)
  if (options.length === 0) return null

  return (
    <div className="relative">
      <button
        onClick={() => setOpen(!open)}
        className={`flex items-center gap-2 px-3 py-2 border rounded-lg text-sm transition-colors ${
          selected.length > 0
            ? 'bg-blue-900/30 border-blue-700 text-blue-300'
            : 'bg-gray-800 border-gray-700 text-gray-300 hover:border-gray-600'
        }`}
      >
        <span>{label}</span>
        {selected.length > 0 && (
          <span className="bg-blue-600 text-white text-xs rounded-full px-1.5 py-0.5 min-w-[1.25rem] text-center">
            {selected.length}
          </span>
        )}
      </button>
      {open && (
        <div className="absolute left-0 mt-1 w-56 bg-gray-800 border border-gray-700 rounded-lg shadow-lg z-20 py-2">
          <div className="px-3 pb-2 border-b border-gray-700 mb-2 flex items-center justify-between">
            <span className="text-xs text-gray-500 uppercase tracking-wider">{label}</span>
            {selected.length > 0 && (
              <button
                onClick={() => onChange([])}
                className="text-xs text-blue-400 hover:text-blue-300"
              >
                Clear
              </button>
            )}
          </div>
          {options.map(opt => (
            <button
              key={opt}
              onClick={() => {
                const next = selected.includes(opt)
                  ? selected.filter(s => s !== opt)
                  : [...selected, opt]
                onChange(next)
              }}
              className={`w-full text-left px-3 py-1.5 text-sm hover:bg-gray-700 flex items-center gap-2 ${
                selected.includes(opt) ? 'text-blue-400' : 'text-gray-300'
              }`}
            >
              <div className={`w-4 h-4 rounded border flex items-center justify-center ${
                selected.includes(opt) ? 'bg-blue-600 border-blue-600' : 'border-gray-600'
              }`}>
                {selected.includes(opt) && <svg className="w-3 h-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={3} d="M5 13l4 4L19 7" />
                </svg>}
              </div>
              {FILTER_LABELS[opt] || opt}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Main Component ─────────────────────────────────────────────

export default function Browse() {
  const [folders, setFolders] = useState<FolderInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [folderScanning, setFolderScanning] = useState(false)
  const [scanStatus, setScanStatus] = useState<MediaScanStatus | null>(null)
  const [error, setError] = useState('')
  const [searchQuery, setSearchQuery] = useState('')
  const [viewMode, setViewMode] = useState<'tree' | 'grid'>('grid')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [sortBy, setSortBy] = useState<SortKey>('name')
  const [showSortMenu, setShowSortMenu] = useState(false)
  const [batchEncoding, setBatchEncoding] = useState(false)
  const [notice, setNotice] = useState<Notice | null>(null)
  const [lastJobID, setLastJobID] = useState<number | null>(null)
  const [preflight, setPreflight] = useState<Record<string, { ok: boolean; message: string }> | null>(null)
  const [jobs, setJobs] = useState<EncodeJob[]>([])
  const [pendingEncode, setPendingEncode] = useState<PendingEncode | null>(null)
  const [selectedProfile, setSelectedProfile] = useState<EncodeProfilePreset>('av1_qsv_balanced28')
  const [creatingJobs, setCreatingJobs] = useState(false)
  const [sourceProbe, setSourceProbe] = useState<SourceProbeSummary | null>(null)
  const [probeLoading, setProbeLoading] = useState(false)
  const [probeError, setProbeError] = useState('')
  const [profileTouched, setProfileTouched] = useState(false)
  const [filters, setFilters] = useState<FilterState>({
    codec: [],
    resolution: [],
    source: [],
    encoded: 'all',
  })

  const preflightFailures = useMemo(() => {
    if (!preflight) return []
    return Object.entries(preflight).filter(([, check]) => !check.ok)
  }, [preflight])

  const loadFolders = useCallback(async (initial = false) => {
    if (initial) setLoading(true)
    try {
      const data = await api.getFolders()
      setFolders(data.folders)
      setScanStatus(data.scan)
      setFolderScanning(data.scan.in_progress && data.folders.length === 0)
      setError('')
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load folders'
      setError(`Failed to load media folders: ${message}`)
    } finally {
      setLoading(false)
    }
  }, [])

  const loadJobs = useCallback(async () => {
    try {
      const data = await api.getJobs()
      setJobs(Array.isArray(data) ? data : [])
    } catch (err) {
      console.error('Failed to load jobs', err)
    }
  }, [])

  useEffect(() => {
    loadFolders(true)
    loadJobs()
    api.getPreflight().then(setPreflight).catch(err => {
      setNotice({ type: 'error', message: err instanceof Error ? err.message : 'Failed to check encoder readiness' })
    })
    const interval = setInterval(loadJobs, 15000)
    return () => clearInterval(interval)
  }, [loadFolders, loadJobs])

  useServerEvents(useCallback((type: string) => {
    if (type.startsWith('media.')) loadFolders(false)
    if (type.startsWith('job.') || type === 'jobs.cleared') loadJobs()
  }, [loadFolders, loadJobs]))

  useEffect(() => {
    if (loading || folders.length > 0 || !scanStatus?.in_progress) return
    const interval = setInterval(() => loadFolders(false), 2000)
    return () => clearInterval(interval)
  }, [folders.length, loadFolders, loading, scanStatus?.in_progress])

  useEffect(() => {
    if (!pendingEncode) {
      setSourceProbe(null)
      setProbeError('')
      setProbeLoading(false)
      return
    }

    let cancelled = false
    setProbeLoading(true)
    setProbeError('')
    setSourceProbe(null)
    api.probeSources(pendingEncode.folders.map(folder => folder.path))
      .then(summary => {
        if (!cancelled) setSourceProbe(summary)
      })
      .catch(err => {
        if (!cancelled) setProbeError(err instanceof Error ? err.message : 'Failed to inspect source')
      })
      .finally(() => {
        if (!cancelled) setProbeLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [pendingEncode])

  const showNotice = useCallback((next: Notice) => {
    setNotice(next)
  }, [])

  const updateFolder = useCallback((nextFolder: FolderInfo) => {
    const replaceFolder = (items: FolderInfo[]): FolderInfo[] => items.map(item => {
      if (item.path === nextFolder.path) return nextFolder
      if (item.children) return { ...item, children: replaceFolder(item.children) }
      return item
    })
    setFolders(prev => replaceFolder(prev))
  }, [])

  const activeJobByPath = useMemo(() => {
    const activeStatuses = new Set(['pending', 'running', 'cancelling'])
    const map = new Map<string, EncodeJob>()
    for (const job of jobs) {
      if (activeStatuses.has(job.status)) {
        map.set(job.source_path, job)
      }
    }
    return map
  }, [jobs])

  const recommendation = useMemo(() => profileRecommendation(sourceProbe, pendingEncode?.folders.length || 1), [pendingEncode?.folders.length, sourceProbe])

  useEffect(() => {
    if (pendingEncode && sourceProbe && !profileTouched) {
      setSelectedProfile(recommendation.preset)
    }
  }, [pendingEncode, profileTouched, recommendation.preset, sourceProbe])

  const openEncodeModal = useCallback((foldersToEncode: FolderInfo[]) => {
    setSelectedProfile(initialProfileForFolders(foldersToEncode))
    setProfileTouched(false)
    setPendingEncode({ folders: foldersToEncode })
  }, [])

  // Flatten to get movie folders (depth 2 typically: media/Movies/Movie Name)
	const movieFolders = useMemo(() => {
		const movies: FolderInfo[] = []
		const traverse = (items: FolderInfo[]) => {
			for (const item of items) {
				if (item.is_dir) {
					if (item.children?.some(c => !c.is_dir) || (!item.children?.length && item.file_count > 0)) {
						movies.push(item)
					}
          if (item.children) traverse(item.children)
        }
      }
    }
    traverse(folders)
    return movies
  }, [folders])

  // Collect all available filter options from actual data
  const availableFilters = useMemo(() => {
    const codecs = new Set<string>()
    const resolutions = new Set<string>()
    const sources = new Set<string>()
    
    for (const movie of movieFolders) {
      const meta = getFolderMeta(movie)
      meta.codecs.forEach(c => codecs.add(c))
      meta.resolutions.forEach(r => resolutions.add(r))
      meta.sources.forEach(s => sources.add(s))
    }
    
    return {
      codecs: Array.from(codecs).sort(),
      resolutions: Array.from(resolutions).sort(),
      sources: Array.from(sources).sort(),
    }
  }, [movieFolders])

  const sortedMovies = useMemo(() => {
    const sorted = [...movieFolders]
    sorted.sort((a, b) => {
      switch (sortBy) {
        case 'size': return b.size - a.size
        case 'fileCount': return b.file_count - a.file_count
        case 'codec': {
          const metaA = getFolderMeta(a)
          const metaB = getFolderMeta(b)
          return (metaA.codecs[0] || '').localeCompare(metaB.codecs[0] || '')
        }
        case 'name': default: return a.name.localeCompare(b.name)
      }
    })
    return sorted
  }, [movieFolders, sortBy])

  const filteredMovies = useMemo(() => {
    let result = sortedMovies
    
    // Search filter
    if (searchQuery) {
      const q = searchQuery.toLowerCase()
      result = result.filter(m =>
        m.name.toLowerCase().includes(q) ||
        m.children?.some(c => c.name.toLowerCase().includes(q))
      )
    }
    
    // Codec filter
    if (filters.codec.length > 0) {
      result = result.filter(m => {
        const meta = getFolderMeta(m)
        return filters.codec.some(c => meta.codecs.includes(c))
      })
    }
    
    // Resolution filter
    if (filters.resolution.length > 0) {
      result = result.filter(m => {
        const meta = getFolderMeta(m)
        return filters.resolution.some(r => meta.resolutions.includes(r))
      })
    }
    
    // Source filter
    if (filters.source.length > 0) {
      result = result.filter(m => {
        const meta = getFolderMeta(m)
        return filters.source.some(s => meta.sources.includes(s))
      })
    }
    
    // Encoded status filter
    if (filters.encoded !== 'all') {
      result = result.filter(m => {
        const meta = getFolderMeta(m)
        if (filters.encoded === 'encoded') return meta.hasEncoded
        if (filters.encoded === 'not-encoded') return meta.hasNonEncoded
        return true
      })
    }
    
    return result
  }, [sortedMovies, searchQuery, filters])

  const maxSize = useMemo(() => {
    return Math.max(...movieFolders.map(m => m.size), 1)
  }, [movieFolders])

  const knownSize = useMemo(() => filteredMovies.reduce((s, m) => s + m.size, 0), [filteredMovies])
  const unknownSizeCount = useMemo(() => filteredMovies.filter(isShallowFolder).length, [filteredMovies])

  const toggleSelect = useCallback((path: string) => {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(path)) next.delete(path)
      else next.add(path)
      return next
    })
  }, [])

  const selectAll = useCallback(() => {
    setSelected(new Set(filteredMovies.map(m => m.path)))
  }, [filteredMovies])

  const deselectAll = useCallback(() => {
    setSelected(new Set())
  }, [])

  const handleBatchEncode = async () => {
    if (selected.size === 0) return
    const foldersToEncode = movieFolders.filter(folder => selected.has(folder.path))
    if (foldersToEncode.length === 0) return
    openEncodeModal(foldersToEncode)
  }

  const startPendingEncode = async () => {
    if (!pendingEncode || creatingJobs) return
    setCreatingJobs(true)
    setBatchEncoding(pendingEncode.folders.length > 1)
    const profile = ENCODE_PROFILES.find(p => p.preset === selectedProfile) || ENCODE_PROFILES[0]
    showNotice({ type: 'info', message: `Creating ${pendingEncode.folders.length} ${profile.label} job${pendingEncode.folders.length === 1 ? '' : 's'}...` })
    const foldersToEncode = pendingEncode.folders.filter(folder => !activeJobByPath.has(folder.path))
    const skippedActive = pendingEncode.folders.length - foldersToEncode.length
    try {
      const result = foldersToEncode.length === 1
        ? await api.createJob(foldersToEncode[0].path, selectedProfile).then(job => ({ created: [{ id: job.id, source_path: foldersToEncode[0].path, status: job.status }], skipped: [], failed: [] }))
        : await api.createJobsBatch(foldersToEncode.map(folder => folder.path), selectedProfile)
      const failures = result.failed.map(item => {
        const folder = foldersToEncode.find(f => f.path === item.source_path)
        return `${folder?.name || item.source_path}: ${item.error}`
      })
      const skipped = skippedActive + result.skipped.length
      if (result.created[0]) setLastJobID(result.created[0].id)
      if (failures.length > 0) {
        showNotice({ type: 'error', message: `Created ${result.created.length}/${foldersToEncode.length} jobs. Failed: ${failures.slice(0, 3).join('; ')}` })
      } else {
        showNotice({ type: 'success', message: `Created ${result.created.length} ${profile.label} job${result.created.length === 1 ? '' : 's'}${skipped > 0 ? `, skipped ${skipped} already active` : ''}. Check Jobs for progress.` })
      }
      setPendingEncode(null)
      setSelected(new Set())
      loadJobs()
    } catch (err) {
      showNotice({ type: 'error', message: err instanceof Error ? err.message : 'Failed to create jobs' })
    } finally {
      setCreatingJobs(false)
      setBatchEncoding(false)
    }
  }

  const handleCreateJob = useCallback(async (folder: FolderInfo) => {
    openEncodeModal([folder])
  }, [openEncodeModal])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-400"></div>
      </div>
    )
  }

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-3xl font-bold">Browse Media</h1>
        <div className="flex items-center gap-3">
          <span className="text-sm text-gray-500">
            {folderScanning && movieFolders.length === 0 ? 'scanning media...' : `${filteredMovies.length} / ${movieFolders.length} movies${knownSize > 0 ? ` · ${formatBytes(knownSize)}` : ''}${unknownSizeCount > 0 ? ' · fast listing' : ''}`}
          </span>
          {(filters.codec.length > 0 || filters.resolution.length > 0 || filters.source.length > 0) && (
            <button
              onClick={() => setFilters({ codec: [], resolution: [], source: [], encoded: 'all' })}
              className="text-xs text-blue-400 hover:text-blue-300 underline"
            >
              Clear all filters
            </button>
          )}
        </div>
      </div>

      {error && (
        <div className="bg-red-900/50 border border-red-700 text-red-200 px-4 py-3 rounded mb-6 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
          <span>{error}</span>
          <button
            onClick={() => loadFolders(true)}
            className="self-start sm:self-auto px-3 py-1.5 bg-red-800 hover:bg-red-700 border border-red-600 rounded text-sm text-red-50"
          >
            Retry
          </button>
        </div>
      )}

      {notice && (
        <div className={`mb-6 border rounded-lg px-4 py-3 flex items-start justify-between gap-4 ${
          notice.type === 'success' ? 'bg-green-900/30 border-green-700 text-green-100' :
          notice.type === 'error' ? 'bg-red-900/40 border-red-700 text-red-100' :
          'bg-blue-900/30 border-blue-700 text-blue-100'
        }`}>
          <div className="flex items-start gap-3 text-sm">
            {notice.type === 'success' ? <CheckCircle className="w-5 h-5 text-green-300 flex-shrink-0" /> :
             notice.type === 'error' ? <AlertCircle className="w-5 h-5 text-red-300 flex-shrink-0" /> :
             <Loader2 className="w-5 h-5 text-blue-300 animate-spin flex-shrink-0" />}
            <div>
              <div>{notice.message}</div>
              {lastJobID && notice.type === 'success' && (
                <a href={`/jobs`} className="text-blue-200 hover:text-blue-100 underline mt-1 inline-block">View Jobs</a>
              )}
            </div>
          </div>
          <button onClick={() => setNotice(null)} className="text-gray-300 hover:text-white">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {scanStatus && (scanStatus.in_progress || scanStatus.last_error || scanStatus.last_completed_at) && (
        <div className="mb-6 rounded-lg border border-gray-700 bg-gray-800/70 px-4 py-3 text-sm text-gray-300">
          {scanStatus.in_progress ? (
            <div className="flex items-center gap-2 text-blue-200">
              <Loader2 className="w-4 h-4 animate-spin" />
              <span>Indexing media{scanStatus.root_path ? ` in ${scanStatus.root_path}` : ''}: {scanStatus.files_indexed} files, {scanStatus.folders_indexed} folders indexed.</span>
            </div>
          ) : scanStatus.last_error ? (
            <div className="text-red-200">Last media scan failed: {scanStatus.last_error}</div>
          ) : (
            <div>Media index ready: {scanStatus.indexed_file_count} files across {scanStatus.indexed_folder_count} folders. Last scanned {scanStatus.last_completed_at}.</div>
          )}
        </div>
      )}

      {preflightFailures.length > 0 && (
        <div className="mb-6 bg-red-950/40 border border-red-800 rounded-lg px-4 py-3 text-red-100">
          <div className="flex items-center gap-2 font-semibold mb-2">
            <AlertCircle className="w-5 h-5 text-red-300" />
            Encoder is not ready
          </div>
          <div className="text-sm text-red-200 space-y-1">
            {preflightFailures.map(([name, check]) => (
              <div key={name}><span className="font-mono">{name}</span>: {check.message}</div>
            ))}
          </div>
        </div>
      )}

      {pendingEncode && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 px-4">
          <div className="w-full max-w-lg rounded-xl border border-gray-700 bg-gray-800 shadow-2xl">
            <div className="border-b border-gray-700 px-5 py-4">
              <h2 className="text-lg font-semibold text-gray-100">Choose Encode Profile</h2>
              <p className="mt-1 text-sm text-gray-400">
                {pendingEncode.folders.length === 1 ? pendingEncode.folders[0].name : `${pendingEncode.folders.length} selected folders`}
              </p>
            </div>
            <div className="space-y-3 px-5 py-4">
              <div className="rounded-lg border border-gray-700 bg-gray-900/60 px-4 py-3">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <div className="text-xs uppercase tracking-wide text-gray-500">Actual Source</div>
                    <div className="mt-1 text-sm font-medium text-gray-100">
                      {probeLoading ? 'Inspecting with ffprobe...' : probeError ? 'Source inspection failed' : summarizeProbe(sourceProbe)}
                    </div>
                  </div>
                  {probeLoading && <Loader2 className="w-5 h-5 animate-spin text-blue-300" />}
                </div>
                {sourceProbe && !probeLoading && (
                  <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-400">
                    <span>{sourceProbe.file_count} file{sourceProbe.file_count === 1 ? '' : 's'}</span>
                    <span>{formatBytes(sourceProbe.total_size)}</span>
                    {estimatedBitrateMbps(sourceProbe) > 0 && <span>{estimatedBitrateMbps(sourceProbe).toFixed(1)} Mbps source</span>}
                    {sourceProbe.files[0]?.duration_seconds > 0 && <span>{formatDuration(sourceProbe.files[0].duration_seconds)}</span>}
                    {sourceProbe.truncated && <span>sampled {sourceProbe.probed_count}</span>}
                  </div>
                )}
                {probeError && <div className="mt-2 text-xs text-red-300">{probeError}</div>}
                {!probeLoading && recommendation.reason && (
                  <div className="mt-2 text-sm text-blue-200">Recommended: {ENCODE_PROFILES.find(profile => profile.preset === recommendation.preset)?.label}. {recommendation.reason}</div>
                )}
                {!probeLoading && recommendation.warning && (
                  <div className="mt-2 rounded border border-yellow-800 bg-yellow-950/30 px-3 py-2 text-sm text-yellow-100">{recommendation.warning}</div>
                )}
              </div>
              {ENCODE_PROFILES.map(profile => (
                <button
                  key={profile.preset}
                  onClick={() => {
                    setProfileTouched(true)
                    setSelectedProfile(profile.preset)
                  }}
                  className={`w-full rounded-lg border px-4 py-3 text-left transition-colors ${
                    selectedProfile === profile.preset
                      ? 'border-blue-500 bg-blue-950/40'
                      : 'border-gray-700 bg-gray-900/50 hover:border-gray-600'
                  }`}
                >
                  <div className="flex items-center justify-between gap-3">
                    <div className="font-medium text-gray-100">{summarizeProbe(sourceProbe)} -&gt; {profile.label}</div>
                    <div className="flex flex-wrap justify-end gap-1.5">
                      {recommendation.preset === profile.preset && <MetaBadge color="green">Recommended</MetaBadge>}
                      {selectedProfile === profile.preset && <MetaBadge color="blue">Selected</MetaBadge>}
                    </div>
                  </div>
                  <div className="mt-1 text-sm text-gray-400">{profile.description}</div>
                </button>
              ))}
              {pendingEncode.folders.some(folder => activeJobByPath.has(folder.path)) && (
                <div className="rounded border border-yellow-800 bg-yellow-950/30 px-3 py-2 text-sm text-yellow-100">
                  Folders with active jobs will be skipped.
                </div>
              )}
            </div>
            <div className="flex items-center justify-end gap-3 border-t border-gray-700 px-5 py-4">
              <button
                onClick={() => setPendingEncode(null)}
                disabled={creatingJobs}
                className="rounded px-4 py-2 text-sm text-gray-300 hover:bg-gray-700 disabled:text-gray-500"
              >
                Cancel
              </button>
              <button
                onClick={startPendingEncode}
                disabled={creatingJobs || pendingEncode.folders.every(folder => activeJobByPath.has(folder.path))}
                className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 disabled:bg-gray-700 disabled:text-gray-500"
              >
                {creatingJobs ? 'Creating...' : 'Start Encode'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Toolbar */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 mb-6">
        {/* Search */}
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
          <input
            type="text"
            placeholder="Search movies..."
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            className="w-full pl-10 pr-10 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-200 placeholder-gray-500 focus:outline-none focus:border-blue-500"
          />
          {searchQuery && (
            <button
              onClick={() => setSearchQuery('')}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300"
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>

        {/* Filters */}
        <div className="flex items-center gap-2">
          {(availableFilters.codecs.length > 0) && (
            <FilterButton
              label="Codec"
              options={availableFilters.codecs}
              selected={filters.codec}
              onChange={codecs => setFilters(prev => ({ ...prev, codec: codecs }))}
            />
          )}
          {(availableFilters.resolutions.length > 0) && (
            <FilterButton
              label="Resolution"
              options={availableFilters.resolutions}
              selected={filters.resolution}
              onChange={res => setFilters(prev => ({ ...prev, resolution: res }))}
            />
          )}
          {(availableFilters.sources.length > 0) && (
            <FilterButton
              label="Source"
              options={availableFilters.sources}
              selected={filters.source}
              onChange={src => setFilters(prev => ({ ...prev, source: src }))}
            />
          )}
        </div>

        {/* Sort */}
        <div className="relative">
          <button
            onClick={() => setShowSortMenu(!showSortMenu)}
            className="flex items-center gap-2 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-300 hover:border-gray-600"
          >
            <ArrowUpDown className="w-4 h-4" />
            Sort: {SORT_OPTIONS.find(s => s.key === sortBy)?.label}
          </button>
          {showSortMenu && (
            <div className="absolute right-0 mt-1 w-40 bg-gray-800 border border-gray-700 rounded-lg shadow-lg z-10">
              {SORT_OPTIONS.map(opt => (
                <button
                  key={opt.key}
                  onClick={() => { setSortBy(opt.key); setShowSortMenu(false) }}
                  className={`w-full text-left px-3 py-2 text-sm hover:bg-gray-700 first:rounded-t-lg last:rounded-b-lg ${sortBy === opt.key ? 'text-blue-400' : 'text-gray-300'}`}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          )}
        </div>

        {/* View toggle */}
        <div className="flex items-center bg-gray-800 border border-gray-700 rounded-lg overflow-hidden">
          <button
            onClick={() => setViewMode('grid')}
            className={`p-2 ${viewMode === 'grid' ? 'bg-gray-700 text-white' : 'text-gray-500 hover:text-gray-300'}`}
          >
            <LayoutGrid className="w-4 h-4" />
          </button>
          <button
            onClick={() => setViewMode('tree')}
            className={`p-2 ${viewMode === 'tree' ? 'bg-gray-700 text-white' : 'text-gray-500 hover:text-gray-300'}`}
          >
            <List className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Batch actions bar */}
      {selected.size > 0 && (
        <div className="flex items-center justify-between bg-blue-900/30 border border-blue-800/50 rounded-lg px-4 py-3 mb-6">
          <div className="flex items-center gap-3">
            <span className="text-sm text-blue-300">{selected.size} selected</span>
            <button
              onClick={selectAll}
              className="text-xs text-blue-400 hover:text-blue-300 underline"
            >
              Select all visible
            </button>
            <button
              onClick={deselectAll}
              className="text-xs text-blue-400 hover:text-blue-300 underline"
            >
              Clear
            </button>
          </div>
          <button
            onClick={handleBatchEncode}
            disabled={batchEncoding}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:bg-gray-700 rounded text-sm font-medium flex items-center gap-2"
          >
            <Play className="w-4 h-4" />
            {batchEncoding ? 'Creating...' : `Encode ${selected.size} selected`}
          </button>
        </div>
      )}

      {/* Content */}
      {viewMode === 'grid' ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {filteredMovies.map(folder => (
            <GridCard
              key={folder.path}
              folder={folder}
              maxSize={maxSize}
              selected={selected}
              onToggleSelect={toggleSelect}
              onEncode={handleCreateJob}
              onFolderUpdate={updateFolder}
              onNotice={showNotice}
              activeJob={activeJobByPath.get(folder.path)}
            />
          ))}
        </div>
      ) : (
        <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
          {/* Tree header */}
          <div className="flex items-center gap-2 px-4 py-2 bg-gray-750 border-b border-gray-700 text-xs text-gray-500 uppercase tracking-wider">
            <span className="w-4" />
            <span className="w-4" />
            <span className="flex-1">Name</span>
            <span className="hidden sm:flex items-center gap-1.5 flex-shrink-0 w-32 justify-end">Meta</span>
            <span className="hidden md:block w-48 text-right">Size</span>
            <span className="w-16 text-right">Files</span>
            <span className="w-16" />
          </div>
          {folders.map(folder => (
            <TreeNode
              key={folder.path}
              folder={folder}
              maxSize={maxSize}
              selected={selected}
              onToggleSelect={toggleSelect}
              onEncode={handleCreateJob}
              searchQuery={searchQuery}
            />
          ))}
        </div>
      )}

      {filteredMovies.length === 0 && !loading && (
        <div className="text-center py-16">
          {folderScanning ? <Loader2 className="w-12 h-12 text-blue-400 mx-auto mb-4 animate-spin" /> : <HardDrive className="w-12 h-12 text-gray-600 mx-auto mb-4" />}
          <p className="text-gray-500">
            {folderScanning ? 'Scanning media folders...' : searchQuery ? 'No movies match your search' : 'No media folders found. Set MEDIA_DIRS environment variable.'}
          </p>
          {folderScanning && <p className="mt-2 text-sm text-gray-600">This should populate automatically when the backend cache is ready.</p>}
        </div>
      )}
    </div>
  )
}
