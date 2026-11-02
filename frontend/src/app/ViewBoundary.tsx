import { Component, type ReactNode } from 'react'
import { Link } from 'react-router-dom'

import { RefreshCw, TriangleAlert } from 'lucide-react'

export class ViewBoundary extends Component<
  { children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  componentDidCatch(error: Error) {
    console.error('URUFLOW view failed', error)
  }

  render() {
    if (!this.state.failed) {
      return this.props.children
    }
    return (
      <section className="view-failure" role="alert">
        <TriangleAlert size={28} aria-hidden="true" />
        <h1>This view couldn’t load</h1>
        <p>Reload to try again, or open another page.</p>
        <div className="page-actions">
          <button className="button" onClick={() => window.location.reload()}>
            <RefreshCw size={15} />
            Reload page
          </button>
          <Link className="button primary" to="/">
            Back to overview
          </Link>
        </div>
      </section>
    )
  }
}
