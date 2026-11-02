import { ErrorState } from '@/shared/ui'

export function PageError() {
  return (
    <main className="page-failure">
      <ErrorState
        message="This page could not be loaded."
        retry={() => window.location.reload()}
      />
    </main>
  )
}
