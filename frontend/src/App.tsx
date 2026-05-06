import { Routes, Route, Link } from 'react-router-dom'
import { HardDrive, ListVideo, BarChart3 } from 'lucide-react'
import Dashboard from './pages/Dashboard'
import Browse from './pages/Browse'
import Jobs from './pages/Jobs'
import Results from './pages/Results'

function App() {
  return (
    <div className="min-h-screen bg-gray-900 text-white">
      <nav className="bg-gray-800 border-b border-gray-700">
        <div className="max-w-7xl mx-auto px-4">
          <div className="flex items-center justify-between h-16">
            <div className="flex items-center gap-2">
              <HardDrive className="w-6 h-6 text-blue-400" />
              <span className="text-xl font-bold">TransGo</span>
            </div>
            <div className="flex gap-6">
              <Link to="/" className="flex items-center gap-2 hover:text-blue-400 transition-colors">
                <BarChart3 className="w-4 h-4" />
                Dashboard
              </Link>
              <Link to="/browse" className="flex items-center gap-2 hover:text-blue-400 transition-colors">
                <HardDrive className="w-4 h-4" />
                Browse
              </Link>
              <Link to="/jobs" className="flex items-center gap-2 hover:text-blue-400 transition-colors">
                <ListVideo className="w-4 h-4" />
                Jobs
              </Link>
            </div>
          </div>
        </div>
      </nav>
      <main className="max-w-7xl mx-auto px-4 py-8">
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/browse" element={<Browse />} />
          <Route path="/jobs" element={<Jobs />} />
          <Route path="/results/:id" element={<Results />} />
        </Routes>
      </main>
    </div>
  )
}

export default App
