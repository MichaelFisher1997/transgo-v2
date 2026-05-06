import { useState, useEffect, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, Save, FileVideo, Trash2, XCircle, Loader2, CheckCircle, AlertCircle } from 'lucide-react'
import { api } from '../api'
import { EncodeResult, EncodeFileResult, EncodeJob, EncodeJobEvent } from '../types'
import { useServerEvents } from '../useServerEvents'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

export default function Results() {
  const { id } = useParams<{ id: string }>()
  const [result, setResult] = useState<EncodeResult | null>(null)
  const [job, setJob] = useState<EncodeJob | null>(null)
  const [files, setFiles] = useState<EncodeFileResult[]>([])
  const [events, setEvents] = useState<EncodeJobEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actionNotice, setActionNotice] = useState<{ type: 'info' | 'success' | 'error'; message: string } | null>(null)
  const [actionLoading, setActionLoading] = useState<'cleanup' | 'discard' | null>(null)
  const [pendingAction, setPendingAction] = useState<{ type: 'cleanup' | 'discard'; count: number } | null>(null)

  useEffect(() => {
    if (id) {
      loadResult(parseInt(id))
      const interval = setInterval(() => loadResult(parseInt(id)), 15000)
      return () => clearInterval(interval)
    }
  }, [id])

  useServerEvents(useCallback((type: string) => {
    if (!id) return
    if (type.startsWith('job.') || type === 'stats.updated') loadResult(parseInt(id))
  }, [id]))

  const loadResult = async (jobId: number) => {
    try {
      const data = await api.getResult(jobId)
      const jobData = await api.getJob(jobId)
      setJob(jobData)
      setResult(data.result)
      setFiles(data.files)
      setEvents(data.events || [])
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load result')
    } finally {
      setLoading(false)
    }
  }

  const handleCleanup = async () => {
    if (!job) return
    setActionLoading('cleanup')
    setPendingAction(null)
    setActionNotice({ type: 'info', message: 'Checking original files eligible for cleanup...' })
    try {
      const preview = await api.cleanupJob(job.id, true)
      if (preview.files.length === 0) {
        setActionNotice({ type: 'info', message: 'No original files are eligible for cleanup.' })
        return
      }
      setPendingAction({ type: 'cleanup', count: preview.files.length })
      setActionNotice({ type: 'info', message: `Ready to delete ${preview.files.length} original file(s) and keep the encoded output. Confirm below to continue.` })
    } catch (err) {
      setActionNotice({ type: 'error', message: err instanceof Error ? err.message : 'Failed to cleanup originals' })
    } finally {
      setActionLoading(null)
    }
  }

  const handleDiscard = async () => {
    if (!job) return
    setActionLoading('discard')
    setPendingAction(null)
    setActionNotice({ type: 'info', message: 'Checking encoded outputs eligible for discard...' })
    try {
      const preview = await api.discardJob(job.id, true)
      if (preview.files.length === 0) {
        setActionNotice({ type: 'info', message: 'No encoded files are eligible for discard.' })
        return
      }
      setPendingAction({ type: 'discard', count: preview.files.length })
      setActionNotice({ type: 'info', message: `Ready to delete ${preview.files.length} encoded output file(s) and keep the original. Confirm below to continue.` })
    } catch (err) {
      setActionNotice({ type: 'error', message: err instanceof Error ? err.message : 'Failed to discard encoded outputs' })
    } finally {
      setActionLoading(null)
    }
  }

  const confirmPendingAction = async () => {
    if (!job || !pendingAction) return
    setActionLoading(pendingAction.type)
    setActionNotice({
      type: 'info',
      message: pendingAction.type === 'cleanup'
        ? `Deleting ${pendingAction.count} original file(s)...`
        : `Deleting ${pendingAction.count} encoded output file(s)...`,
    })
    try {
      if (pendingAction.type === 'cleanup') {
        const result = await api.cleanupJob(job.id)
        setActionNotice({ type: result.failed > 0 ? 'error' : 'success', message: `Deleted ${result.deleted} original file(s). ${result.failed} failed.` })
      } else {
        const result = await api.discardJob(job.id)
        setActionNotice({ type: result.failed > 0 ? 'error' : 'success', message: `Deleted ${result.deleted} encoded output file(s). ${result.missing} already missing, ${result.failed} failed.` })
      }
      setPendingAction(null)
      await loadResult(job.id)
    } catch (err) {
      setActionNotice({ type: 'error', message: err instanceof Error ? err.message : 'Finalization action failed' })
    } finally {
      setActionLoading(null)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-400"></div>
      </div>
    )
  }

  if (!result) {
    return (
      <div className="text-center py-8">
        <p className="text-gray-400">Result not found</p>
        <Link to="/jobs" className="text-blue-400 hover:underline mt-4 inline-block">
          Back to Jobs
        </Link>
      </div>
    )
  }

  const savingsPercent = result.original_folder_size > 0
    ? ((result.space_saved / result.original_folder_size) * 100).toFixed(1)
    : '0'

  const formatDuration = (seconds: number): string => {
    if (!seconds) return '-'
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    const s = Math.floor(seconds % 60)
    if (h > 0) return `${h}h ${m}m`
    if (m > 0) return `${m}m ${s}s`
    return `${s}s`
  }

  return (
    <div>
      <div className="flex items-center gap-4 mb-8">
        <Link to="/jobs" className="text-gray-400 hover:text-white">
          <ArrowLeft className="w-6 h-6" />
        </Link>
        <h1 className="text-3xl font-bold">Encode Results</h1>
      </div>

      {error && (
        <div className="bg-red-900/50 border border-red-700 text-red-200 px-4 py-3 rounded mb-6">
          {error}
        </div>
      )}

      {actionNotice && (
        <div className={`border px-4 py-3 rounded mb-6 flex flex-col gap-3 ${
          actionNotice.type === 'success' ? 'bg-green-900/30 border-green-700 text-green-100' :
          actionNotice.type === 'error' ? 'bg-red-900/40 border-red-700 text-red-100' :
          'bg-blue-900/30 border-blue-700 text-blue-100'
        }`}>
          <div className="flex items-center gap-3 text-sm">
            {actionNotice.type === 'success' ? <CheckCircle className="w-5 h-5 text-green-300 flex-shrink-0" /> :
             actionNotice.type === 'error' ? <AlertCircle className="w-5 h-5 text-red-300 flex-shrink-0" /> :
             actionLoading ? <Loader2 className="w-5 h-5 text-blue-300 animate-spin flex-shrink-0" /> : <AlertCircle className="w-5 h-5 text-blue-300 flex-shrink-0" />}
            <span>{actionNotice.message}</span>
          </div>
          {pendingAction && (
            <div className="flex flex-wrap gap-2">
              <button
                onClick={confirmPendingAction}
                disabled={actionLoading !== null}
                className={`px-3 py-1.5 rounded text-sm font-medium flex items-center gap-2 ${pendingAction.type === 'cleanup' ? 'bg-green-600 hover:bg-green-500' : 'bg-red-600 hover:bg-red-500'} disabled:bg-gray-700 disabled:text-gray-500`}
              >
                {actionLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : pendingAction.type === 'cleanup' ? <Trash2 className="w-4 h-4" /> : <XCircle className="w-4 h-4" />}
                Confirm {pendingAction.type === 'cleanup' ? 'Delete Originals' : 'Delete Encoded'}
              </button>
              <button
                onClick={() => {
                  setPendingAction(null)
                  setActionNotice({ type: 'info', message: 'Finalization cancelled. No files were deleted.' })
                }}
                disabled={actionLoading !== null}
                className="px-3 py-1.5 rounded text-sm bg-gray-700 hover:bg-gray-600 disabled:bg-gray-800 disabled:text-gray-500"
              >
                Cancel
              </button>
            </div>
          )}
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
          <div className="flex items-center gap-3 mb-2">
            <FileVideo className="w-8 h-8 text-gray-400" />
            <div>
              <p className="text-sm text-gray-400">Original Size</p>
              <p className="text-2xl font-bold">{formatBytes(result.original_folder_size)}</p>
            </div>
          </div>
        </div>

        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
          <div className="flex items-center gap-3 mb-2">
            <FileVideo className="w-8 h-8 text-blue-400" />
            <div>
              <p className="text-sm text-gray-400">Encoded Size</p>
              <p className="text-2xl font-bold">{formatBytes(result.encoded_folder_size)}</p>
            </div>
          </div>
        </div>

        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
          <div className="flex items-center gap-3 mb-2">
            <Save className="w-8 h-8 text-green-400" />
            <div>
              <p className="text-sm text-gray-400">Space Saved</p>
              <p className="text-2xl font-bold text-green-400">
                {formatBytes(result.space_saved)} ({savingsPercent}%)
              </p>
            </div>
          </div>
        </div>
      </div>

      {job && (job.status === 'completed' || job.status === 'completed_with_errors') && (
        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700 mb-8">
          <h2 className="text-xl font-bold mb-2">Finalize Files</h2>
          <p className="text-sm text-gray-400 mb-4">
            Cleanup keeps the encoded output and deletes the original. Discard keeps the original and deletes the encoded output.
          </p>
          <div className="flex flex-wrap gap-3">
            <button
              onClick={handleCleanup}
              disabled={actionLoading !== null || pendingAction !== null || job.cleanup_status === 'completed' || job.cleanup_status === 'discarded'}
              className="px-4 py-2 bg-green-600 hover:bg-green-500 disabled:bg-gray-700 disabled:text-gray-500 rounded text-sm font-medium flex items-center gap-2"
            >
              {actionLoading === 'cleanup' ? <Loader2 className="w-4 h-4 animate-spin" /> : <Trash2 className="w-4 h-4" />}
              {actionLoading === 'cleanup' ? 'Cleaning...' : 'Cleanup: delete originals'}
            </button>
            <button
              onClick={handleDiscard}
              disabled={actionLoading !== null || pendingAction !== null || job.cleanup_status === 'completed' || job.cleanup_status === 'discarded'}
              className="px-4 py-2 bg-red-600 hover:bg-red-500 disabled:bg-gray-700 disabled:text-gray-500 rounded text-sm font-medium flex items-center gap-2"
            >
              {actionLoading === 'discard' ? <Loader2 className="w-4 h-4 animate-spin" /> : <XCircle className="w-4 h-4" />}
              {actionLoading === 'discard' ? 'Discarding...' : 'Discard: delete encoded'}
            </button>
            {job.cleanup_status !== 'not_started' && (
              <span className="text-sm text-gray-400 self-center">Current finalization: {job.cleanup_status}</span>
            )}
          </div>
        </div>
      )}

      {job && (job.status === 'running' || job.current_file) && (
        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700 mb-8">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-xl font-bold">Live Progress</h2>
            <span className="text-sm text-gray-400">Overall {job.progress_percent.toFixed(1)}%</span>
          </div>
          <div className="w-full bg-gray-700 rounded-full h-2 mb-4">
            <div className="bg-blue-500 h-2 rounded-full transition-all" style={{ width: `${Math.min(job.progress_percent, 100)}%` }} />
          </div>
          <div className="grid grid-cols-2 md:grid-cols-5 gap-4 text-sm">
            <div><div className="text-gray-500">File</div><div className="truncate" title={job.current_file}>{job.current_file.split('/').pop() || '-'}</div></div>
            <div><div className="text-gray-500">File %</div><div>{job.current_file_percent.toFixed(1)}%</div></div>
            <div><div className="text-gray-500">Frames</div><div>{job.processed_frames.toLocaleString()} / {job.total_frames ? job.total_frames.toLocaleString() : '?'}</div></div>
            <div><div className="text-gray-500">FPS / Speed</div><div>{job.fps ? job.fps.toFixed(1) : '-'} fps · {job.speed || '-'}</div></div>
            <div><div className="text-gray-500">ETA</div><div>{formatDuration(job.eta_seconds)}</div></div>
          </div>
        </div>
      )}

      <h2 className="text-xl font-bold mb-4">File Comparisons</h2>
      <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-750 border-b border-gray-700">
            <tr>
              <th className="text-left px-4 py-3 text-sm font-medium text-gray-400">Paths</th>
              <th className="text-right px-4 py-3 text-sm font-medium text-gray-400">Original</th>
              <th className="text-right px-4 py-3 text-sm font-medium text-gray-400">Encoded</th>
              <th className="text-right px-4 py-3 text-sm font-medium text-gray-400">Saved</th>
              <th className="text-left px-4 py-3 text-sm font-medium text-gray-400">Status</th>
            </tr>
          </thead>
          <tbody>
            {files.map((file) => (
              <tr key={file.id} className="border-b border-gray-700/50">
                <td className="px-4 py-3 text-sm">
                  <div className="max-w-xl space-y-1">
                    <div className="truncate" title={file.original_path}>
                      <span className="text-gray-500">Old:</span> {file.original_path}
                    </div>
                    <div className="truncate" title={file.encoded_path}>
                      <span className="text-gray-500">New:</span> {file.encoded_path}
                    </div>
                  </div>
                </td>
                <td className="px-4 py-3 text-sm text-right">{formatBytes(file.original_size)}</td>
                <td className="px-4 py-3 text-sm text-right">
                  {file.status === 'success' ? formatBytes(file.encoded_size) : '-'}
                </td>
                <td className="px-4 py-3 text-sm text-right">
                  {file.status === 'success' ? (
                    <span className="text-green-400">
                      {formatBytes(file.space_saved)}
                    </span>
                  ) : (
                    '-'
                  )}
                </td>
                <td className="px-4 py-3">
                  <span className={`text-xs px-2 py-1 rounded ${
                    file.status === 'success' ? 'bg-green-900/50 text-green-200' :
                    file.status === 'failed' ? 'bg-red-900/50 text-red-200' :
                    file.status === 'skipped' ? 'bg-gray-700 text-gray-200' :
                    'bg-yellow-900/50 text-yellow-200'
                  }`}>
                    {file.status}
                  </span>
                  {file.cleanup_status !== 'not_started' && (
                    <div className="text-xs text-gray-400 mt-1">{file.cleanup_status}</div>
                  )}
                  {file.error_message && (
                    <div className="text-xs text-red-300 mt-1 max-w-sm" title={file.ffmpeg_log_tail || file.error_message}>
                      {file.error_message}
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {events.length > 0 && (
        <div className="mt-8">
          <h2 className="text-xl font-bold mb-4">Job Log</h2>
          <div className="bg-gray-900 border border-gray-700 rounded-lg divide-y divide-gray-800">
            {events.map(event => (
              <div key={event.id} className="px-4 py-3 text-sm">
                <div className="flex items-center gap-3">
                  <span className={`uppercase text-xs ${event.level === 'error' ? 'text-red-400' : event.level === 'warn' ? 'text-orange-400' : 'text-blue-400'}`}>
                    {event.level}
                  </span>
                  <span className="text-gray-500">{new Date(event.created_at).toLocaleString()}</span>
                </div>
                <div className="text-gray-200 mt-1">{event.message}</div>
                {event.file_path && <div className="text-gray-500 text-xs truncate">{event.file_path}</div>}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
