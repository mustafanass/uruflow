import { AlertCircle } from 'lucide-react'

export function ErrorState({
  message,
  retry,
}: {
  message: string
  retry?: () => void
}) {
  return (
    <div className="error-panel" role="alert">
      <AlertCircle size={18} />
      <div>
        <strong>Could not load this view</strong>
        <p>{message}</p>
        {retry && (
          <button className="text-button" onClick={retry}>
            Try again
          </button>
        )}
      </div>
    </div>
  )
}
