export function LoadingIndicator({ small = false }: { small?: boolean }) {
  return (
    <span
      className={'loading-indicator' + (small ? ' small' : '')}
      aria-hidden="true"
    >
      <svg viewBox="0 0 40 40" fill="none">
        <circle
          className="loading-track"
          cx="20"
          cy="20"
          r="17"
          strokeWidth="2.5"
        />
        <circle
          className="loading-arc"
          cx="20"
          cy="20"
          r="17"
          strokeWidth="2.5"
          strokeLinecap="round"
          strokeDasharray="32 75"
        />
      </svg>
      {!small && <img src="/uruflow-logo.png" alt="" width="22" height="22" />}
    </span>
  )
}
