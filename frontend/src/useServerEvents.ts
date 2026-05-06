import { useEffect } from 'react'

export function useServerEvents(onEvent: (type: string) => void) {
  useEffect(() => {
    const source = new EventSource('/api/events')
    const eventTypes = [
      'job.created',
      'job.queued',
      'job.updated',
      'job.progress',
      'job.completed',
      'job.deleted',
      'job.event',
      'jobs.cleared',
      'stats.updated',
      'settings.updated',
      'media.scan.started',
      'media.scan.progress',
      'media.scan.completed',
      'media.scan.failed',
      'media.folder.updated',
    ]
    const handlers = eventTypes.map(type => {
      const handler = () => onEvent(type)
      source.addEventListener(type, handler)
      return { type, handler }
    })
    source.onerror = () => {
      // EventSource reconnects automatically; snapshots still reload on each next event.
    }
    return () => {
      handlers.forEach(({ type, handler }) => source.removeEventListener(type, handler))
      source.close()
    }
  }, [onEvent])
}
