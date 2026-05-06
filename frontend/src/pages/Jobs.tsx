import { useState, useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { Play, CheckCircle, XCircle, Clock, AlertCircle, Trash2, Loader2, X } from 'lucide-react'
import { api } from '../api'
import { EncodeJob } from '../types'
import { useServerEvents } from '../useServerEvents'

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    pending: 'bg-yellow-900/50 text-yellow-200 border-yellow-700',
    running: 'bg-blue-900/50 text-blue-200 border-blue-700',
    completed: 'bg-green-900/50 text-green-200 border-green-700',
    completed_with_errors: 'bg-orange-900/50 text-orange-200 border-orange-700',
    skipped: 'bg-gray-800 text-gray-300 border-gray-600',
    failed: 'bg-red-900/50 text-red-200 border-red-700',
    cancelling: 'bg-orange-900/50 text-orange-200 border-orange-700',
    cancelled: 'bg-gray-800 text-gray-300 border-gray-600',
  }

  const icons: Record<string, React.ReactNode> = {
    pending: <Clock className="w-3 h-3" />,
    running: <Play className="w-3 h-3" />,
    completed: <CheckCircle className="w-3 h-3" />,
    completed_with_errors: <AlertCircle className="w-3 h-3" />,
    failed: <AlertCircle className="w-3 h-3" />,
    cancelling: <Clock className="w-3 h-3" />,
    cancelled: <XCircle className="w-3 h-3" />,
  }

  return (
    <span className={`inline-flex items-center gap-1 px-2 py-1 rounded text-xs border ${styles[status] || styles.pending}`}>
      {icons[status] || icons.pending}
      {status}
    </span>
  )
}

function formatDuration(seconds: number): string {
  if (!seconds) return '-'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

export default function Jobs() {
  const [jobs, setJobs] = useState<EncodeJob[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState<{ type: 'success' | 'error' | 'info'; message: string } | null>(null)
  const [clearingAll, setClearingAll] = useState(false)
  const [clearingJobID, setClearingJobID] = useState<number | null>(null)
  const [pendingClearJobID, setPendingClearJobID] = useState<number | null>(null)

  useEffect(() => {
    loadJobs()
    const interval = setInterval(loadJobs, 15000)
    return () => clearInterval(interval)
  }, [])

  useServerEvents(useCallback((type: string) => {
    if (type.startsWith('job.') || type === 'jobs.cleared') loadJobs()
  }, []))

  const loadJobs = async () => {
    try {
      const data = await api.getJobs()
      setJobs(Array.isArray(data) ? data : [])
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load jobs')
    } finally {
      setLoading(false)
    }
  }

  const handleCancel = async (id: number) => {
    try {
      await api.cancelJob(id)
      loadJobs()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to cancel job')
    }
  }

  const handleRetry = async (job: EncodeJob) => {
    if (!confirm(`Retry job #${job.id}? This will create a new pending job for the same source.`)) return
    try {
      const result = await api.retryJob(job.id)
      alert(`Created retry job #${result.id}`)
      loadJobs()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to retry job')
    }
  }

  const handleCleanup = async (id: number) => {
    const preview = await api.cleanupJob(id, true)
    if (!confirm(`This will delete ${preview.files.length} original files. This cannot be undone. Continue?`)) return
    try {
      const result = await api.cleanupJob(id)
      alert(`Deleted ${result.deleted} files, ${result.failed} failed`)
      loadJobs()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to cleanup')
    }
  }

  const handleDelete = async (id: number) => {
    setPendingClearJobID(id)
    setNotice({ type: 'info', message: `Ready to clear job #${id}. Click Confirm Clear to remove it from this list. Media files are kept; incomplete TransGo temp outputs for this job will be deleted.` })
  }

  const confirmDelete = async (id: number) => {
    setClearingJobID(id)
    setPendingClearJobID(null)
    setNotice({ type: 'info', message: `Clearing job #${id}...` })
    setJobs(prev => prev.filter(job => job.id !== id))
    try {
      const result = await api.deleteJob(id)
      setNotice({ type: result.temp_failed > 0 ? 'error' : 'success', message: `Cleared job #${id}. Deleted ${result.temp_deleted} incomplete temp file(s), ${result.temp_failed} failed.` })
      await loadJobs()
    } catch (err) {
      setNotice({ type: 'error', message: err instanceof Error ? err.message : 'Failed to clear job' })
    } finally {
      setClearingJobID(null)
    }
  }

  const handleClearAll = async () => {
    const clearable = jobs.filter(job => job.status !== 'running' && job.status !== 'pending')
    const active = jobs.length - clearable.length
    if (clearable.length === 0) {
      setNotice({ type: 'info', message: active > 0 ? 'Only active jobs remain. Cancel running jobs before clearing them.' : 'No jobs to clean.' })
      return
    }
    setClearingAll(true)
    setNotice({ type: 'info', message: `Cleaning ${clearable.length} non-running job${clearable.length === 1 ? '' : 's'}${active > 0 ? ` and skipping ${active} active` : ''}...` })
    setJobs(prev => prev.filter(job => job.status === 'running' || job.status === 'pending'))
    try {
      const result = await api.clearJobs()
      setNotice({ type: result.temp_failed > 0 ? 'error' : 'success', message: `Cleared ${result.cleared} job(s), skipped ${result.skipped}. Deleted ${result.temp_deleted} incomplete temp file(s), ${result.temp_failed} failed.` })
      await loadJobs()
    } catch (err) {
      setNotice({ type: 'error', message: err instanceof Error ? err.message : 'Failed to clear jobs' })
      await loadJobs()
    } finally {
      setClearingAll(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-400"></div>
      </div>
    )
  }

  return (
    <div>
      <div className="mb-8 flex items-center justify-between gap-4">
        <h1 className="text-3xl font-bold">Encoding Jobs</h1>
        {jobs.some(job => job.status !== 'running' && job.status !== 'pending') && (
          <button
            onClick={handleClearAll}
            disabled={clearingAll}
            className="px-3 py-2 bg-gray-700 hover:bg-gray-600 disabled:bg-gray-800 disabled:text-gray-500 rounded text-sm flex items-center gap-2"
          >
            {clearingAll ? <Loader2 className="w-4 h-4 animate-spin" /> : <Trash2 className="w-4 h-4" />}
            {clearingAll ? 'Cleaning...' : 'Clean All'}
          </button>
        )}
      </div>
      
      {error && (
        <div className="bg-red-900/50 border border-red-700 text-red-200 px-4 py-3 rounded mb-6">
          {error}
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
            <div>{notice.message}</div>
          </div>
          <button onClick={() => setNotice(null)} className="text-gray-300 hover:text-white">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-750 border-b border-gray-700">
            <tr>
              <th className="text-left px-4 py-3 text-sm font-medium text-gray-400">ID</th>
              <th className="text-left px-4 py-3 text-sm font-medium text-gray-400">Source</th>
              <th className="text-left px-4 py-3 text-sm font-medium text-gray-400">Status</th>
              <th className="text-left px-4 py-3 text-sm font-medium text-gray-400">Progress</th>
              <th className="text-left px-4 py-3 text-sm font-medium text-gray-400">Preset</th>
              <th className="text-left px-4 py-3 text-sm font-medium text-gray-400">Actions</th>
            </tr>
          </thead>
          <tbody>
            {jobs.length === 0 ? (
              <tr>
                <td colSpan={6} className="text-center py-8 text-gray-400">
                  No encoding jobs yet. Go to Browse to create one.
                </td>
              </tr>
            ) : (
              jobs.map((job) => (
                <tr key={job.id} className="border-b border-gray-700/50 hover:bg-gray-750">
                  <td className="px-4 py-3">{job.id}</td>
                  <td className="px-4 py-3 text-sm">
                    <div className="max-w-xs truncate" title={job.source_path}>
                      {job.source_path}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={job.status} />
                    {job.error_message && (
                      <div className="text-xs text-red-300 mt-1 max-w-xs" title={job.error_message}>
                        {job.error_message}
                      </div>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <div className="w-24 bg-gray-700 rounded-full h-2">
                        <div
                          className="bg-blue-500 h-2 rounded-full transition-all"
                          style={{
                            width: `${job.progress_percent || (job.total_files > 0 ? (job.completed_files / job.total_files) * 100 : 0)}%`,
                          }}
                        />
                      </div>
                      <span className="text-xs text-gray-400">
                        {job.completed_files}/{job.total_files}
                      </span>
                    </div>
                    {job.current_file && (
                      <div className="text-xs text-gray-500 max-w-xs mt-1" title={job.current_file}>
                        <div className="truncate">{job.current_file.split('/').pop()}</div>
                        <div className="flex flex-wrap gap-x-2 gap-y-1 mt-1">
                          <span>{job.current_file_percent.toFixed(1)}%</span>
                          <span>{job.processed_frames.toLocaleString()}/{job.total_frames ? job.total_frames.toLocaleString() : '?'} frames</span>
                          <span>{job.fps ? `${job.fps.toFixed(1)} fps` : 'fps -'}</span>
                          <span>{job.speed || 'speed -'}</span>
                          <span>ETA {formatDuration(job.eta_seconds)}</span>
                          {job.ffmpeg_pid > 0 && <span>PID {job.ffmpeg_pid}</span>}
                        </div>
                      </div>
                    )}
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-400">{job.preset}</td>
                  <td className="px-4 py-3">
                    <div className="flex gap-2">
                      {(job.status === 'running' || job.status === 'pending') && (
                        <button
                          onClick={() => handleCancel(job.id)}
                          className="px-2 py-1 bg-red-600 hover:bg-red-500 rounded text-xs"
                        >
                          Cancel
                        </button>
                      )}
                      {(job.status === 'completed' || job.status === 'completed_with_errors' || job.status === 'failed' || job.status === 'cancelled' || job.status === 'cancelling') && (
                        <>
                          {(job.status === 'completed' || job.status === 'completed_with_errors') && (
                            <Link
                              to={`/results/${job.id}`}
                              className="px-2 py-1 bg-blue-600 hover:bg-blue-500 rounded text-xs"
                            >
                              Results
                            </Link>
                          )}
                          {job.status === 'completed' && job.cleanup_status !== 'completed' && (
                            <button
                              onClick={() => handleCleanup(job.id)}
                              className="px-2 py-1 bg-yellow-600 hover:bg-yellow-500 rounded text-xs flex items-center gap-1"
                            >
                              <Trash2 className="w-3 h-3" />
                              Cleanup
                            </button>
                          )}
                          {(job.status === 'failed' || job.status === 'cancelled' || job.status === 'completed_with_errors') && (
                            <button
                              onClick={() => handleRetry(job)}
                              className="px-2 py-1 bg-green-700 hover:bg-green-600 rounded text-xs flex items-center gap-1"
                            >
                              <Play className="w-3 h-3" />
                              Retry
                            </button>
                          )}
                          {(job.status === 'completed' || job.status === 'failed' || job.status === 'completed_with_errors' || job.status === 'cancelled' || job.status === 'cancelling') && (
                            pendingClearJobID === job.id ? (
                              <>
                                <button
                                  onClick={() => confirmDelete(job.id)}
                                  disabled={clearingJobID === job.id || clearingAll}
                                  className="px-2 py-1 bg-red-600 hover:bg-red-500 disabled:bg-gray-800 disabled:text-gray-500 rounded text-xs flex items-center gap-1"
                                >
                                  {clearingJobID === job.id ? <Loader2 className="w-3 h-3 animate-spin" /> : <Trash2 className="w-3 h-3" />}
                                  {clearingJobID === job.id ? 'Clearing' : 'Confirm Clear'}
                                </button>
                                <button
                                  onClick={() => {
                                    setPendingClearJobID(null)
                                    setNotice({ type: 'info', message: `Clear cancelled for job #${job.id}.` })
                                  }}
                                  disabled={clearingJobID === job.id || clearingAll}
                                  className="px-2 py-1 bg-gray-700 hover:bg-gray-600 disabled:bg-gray-800 disabled:text-gray-500 rounded text-xs"
                                >
                                  Cancel
                                </button>
                              </>
                            ) : (
                              <button
                                onClick={() => handleDelete(job.id)}
                                disabled={clearingJobID === job.id || clearingAll}
                                className="px-2 py-1 bg-gray-700 hover:bg-gray-600 disabled:bg-gray-800 disabled:text-gray-500 rounded text-xs flex items-center gap-1"
                              >
                                {clearingJobID === job.id ? <Loader2 className="w-3 h-3 animate-spin" /> : <Trash2 className="w-3 h-3" />}
                                {clearingJobID === job.id ? 'Clearing' : 'Clear'}
                              </button>
                            )
                          )}
                        </>
                      )}
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
