import { useEffect, useState } from 'react'

import { Check, Copy } from 'lucide-react'

import { useWorkspace } from '@/shared/hooks'

export function CopyButton({
  value,
  label = 'Copy',
}: {
  value: string
  label?: string
}) {
  const [copied, setCopied] = useState(false)
  const { notify } = useWorkspace()
  useEffect(() => {
    if (copied) {
      const t = setTimeout(() => setCopied(false), 1800)
      return () => clearTimeout(t)
    }
  }, [copied])
  return (
    <button
      className="icon-button"
      title={label}
      aria-label={copied ? 'Copied' : label}
      onClick={() =>
        navigator.clipboard
          .writeText(value)
          .then(() => setCopied(true))
          .catch(() =>
            notify(
              'Clipboard access is unavailable. Select and copy the value.',
              'error',
            ),
          )
      }
    >
      {copied ? <Check size={15} /> : <Copy size={15} />}
    </button>
  )
}
export function Code({
  value,
  copy = false,
}: {
  value: string
  copy?: boolean
}) {
  return (
    <span className="code-value">
      <code>{value || '—'}</code>
      {copy && value && <CopyButton value={value} />}
    </span>
  )
}
