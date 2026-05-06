import { useState, useEffect, useCallback } from 'react'
import { Play, CheckCircle, Clock, Save, Loader2 } from 'lucide-react'
import { api } from '../api'
import { AppSettings, Stats } from '../types'
import { useServerEvents } from '../useServerEvents'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

export default function Dashboard() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [settings, setSettings] = useState<AppSettings | null>(null)
  const [preflight, setPreflight] = useState<Record<string, { ok: boolean; message: string }> | null>(null)
  const [loading, setLoading] = useState(true)
  const [savingSettings, setSavingSettings] = useState(false)
  const [settingsNotice, setSettingsNotice] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    loadStats()
    const interval = setInterval(loadStats, 15000)
    return () => clearInterval(interval)
  }, [])

  useServerEvents(useCallback((type: string) => {
    if (type.startsWith('job.') || type === 'jobs.cleared' || type === 'stats.updated' || type === 'settings.updated') loadStats()
  }, []))

  const loadStats = async () => {
    try {
      const data = await api.getStats()
      const appSettings = await api.getSettings()
      const checks = await api.getPreflight()
      setStats(data)
      setSettings(appSettings)
      setPreflight(checks)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load stats')
    } finally {
      setLoading(false)
    }
  }

  const updateConcurrency = async (value: number) => {
    if (!settings || savingSettings) return
    setSavingSettings(true)
    setSettingsNotice(`Saving max jobs: ${value}...`)
    try {
      const next = await api.updateSettings({ encoder_concurrency: value })
      setSettings(next)
      setSettingsNotice(`Max concurrent jobs set to ${next.encoder_concurrency}. Running jobs are not cancelled; queued jobs obey the new limit.`)
    } catch (err) {
      setSettingsNotice(err instanceof Error ? err.message : 'Failed to update max jobs')
    } finally {
      setSavingSettings(false)
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
      <h1 className="text-3xl font-bold mb-8">Dashboard</h1>
      
      {error && (
        <div className="bg-red-900/50 border border-red-700 text-red-200 px-4 py-3 rounded mb-6">
          {error}
        </div>
      )}

      {stats && (
        <>
        <div className="mb-8 rounded-xl border border-green-700/50 bg-gradient-to-br from-green-950/60 to-gray-800 p-6">
          <div className="flex items-center gap-4">
            <div className="rounded-lg bg-green-500/10 p-3">
              <Save className="h-10 w-10 text-green-300" />
            </div>
            <div>
              <p className="text-sm uppercase tracking-wide text-green-300">Total Space Saved So Far</p>
              <p className="text-4xl font-bold text-white">{formatBytes(stats.space_freed)}</p>
              <p className="mt-1 text-sm text-gray-400">Counts only old original files deleted through cleanup.</p>
            </div>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
          <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
            <div className="flex items-center gap-3 mb-2">
              <Save className="w-8 h-8 text-green-400" />
              <div>
                <p className="text-sm text-gray-400">Space Freed</p>
                <p className="text-2xl font-bold">{formatBytes(stats.space_freed)}</p>
                <p className="text-xs text-gray-500 mt-1">after manual cleanup</p>
              </div>
            </div>
          </div>

          <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
            <div className="flex items-center gap-3 mb-2">
              <Play className="w-8 h-8 text-blue-400" />
              <div>
                <p className="text-sm text-gray-400">Potential Savings</p>
                <p className="text-2xl font-bold">{formatBytes(stats.potential_space_saved ?? stats.total_space_saved)}</p>
                <p className="text-xs text-gray-500 mt-1">encoded, not necessarily deleted</p>
              </div>
            </div>
          </div>

          <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
            <div className="flex items-center gap-3 mb-2">
              <CheckCircle className="w-8 h-8 text-green-400" />
              <div>
                <p className="text-sm text-gray-400">Completed</p>
                <p className="text-2xl font-bold">{stats.completed_jobs}</p>
              </div>
            </div>
          </div>

          <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
            <div className="flex items-center gap-3 mb-2">
              <Clock className="w-8 h-8 text-yellow-400" />
              <div>
                <p className="text-sm text-gray-400">Total Jobs</p>
                <p className="text-2xl font-bold">{stats.total_jobs}</p>
              </div>
            </div>
          </div>
        </div>

        {settings && (
          <div className="mt-8 bg-gray-800 rounded-lg border border-gray-700 p-6">
            <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
              <div>
                <h2 className="text-xl font-bold">Encoder Settings</h2>
                <p className="mt-1 text-sm text-gray-400">Controls how many FFmpeg jobs TransGo runs at the same time.</p>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                {[1, 2, 3, 4, 6, 8].map(value => (
                  <button
                    key={value}
                    onClick={() => updateConcurrency(value)}
                    disabled={savingSettings || settings.encoder_concurrency === value}
                    className={`px-3 py-2 rounded text-sm border flex items-center gap-2 ${settings.encoder_concurrency === value ? 'bg-blue-600 border-blue-500 text-white' : 'bg-gray-700 border-gray-600 hover:bg-gray-600 disabled:bg-gray-800 disabled:text-gray-500'}`}
                  >
                    {savingSettings && settings.encoder_concurrency !== value ? null : settings.encoder_concurrency === value ? <CheckCircle className="w-4 h-4" /> : null}
                    {value} job{value === 1 ? '' : 's'}
                  </button>
                ))}
              </div>
            </div>
            <div className="mt-4 flex items-center gap-2 text-sm text-gray-400">
              {savingSettings && <Loader2 className="w-4 h-4 animate-spin text-blue-300" />}
              <span>{settingsNotice || `Current max: ${settings.encoder_concurrency} concurrent job${settings.encoder_concurrency === 1 ? '' : 's'}.`}</span>
            </div>
          </div>
        )}
        </>
      )}

      {preflight && (
        <div className="mt-8 bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
          <div className="px-4 py-3 border-b border-gray-700 font-semibold">Encoder Preflight</div>
          <div className="divide-y divide-gray-700">
            {Object.entries(preflight).map(([name, check]) => (
              <div key={name} className="px-4 py-3 flex items-center justify-between gap-4">
                <div>
                  <div className="text-sm font-medium">{name}</div>
                  <div className="text-xs text-gray-500">{check.message}</div>
                </div>
                <span className={`text-xs px-2 py-1 rounded ${check.ok ? 'bg-green-900/50 text-green-200' : 'bg-red-900/50 text-red-200'}`}>
                  {check.ok ? 'OK' : 'FAIL'}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
