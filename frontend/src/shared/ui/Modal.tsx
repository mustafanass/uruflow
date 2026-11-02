import { type ReactNode, useEffect, useId, useRef } from 'react'
import { createPortal } from 'react-dom'

import { X } from 'lucide-react'

export function Modal({
  title,
  children,
  onClose,
  wide = false,
  locked = false,
}: {
  title: string
  children: ReactNode
  onClose: () => void
  wide?: boolean
  locked?: boolean
}) {
  const id = useId()
  const ref = useRef<HTMLDivElement>(null)
  const close = useRef(onClose)
  const preventClose = useRef(locked)
  close.current = onClose
  preventClose.current = locked
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null
    const first = ref.current?.querySelector<HTMLElement>(
      'input:not(:disabled), textarea:not(:disabled), select:not(:disabled), [data-autofocus], .modal-actions button:not(:disabled)',
    )
    ;(first || ref.current)?.focus()
    const handler = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !preventClose.current) {
        close.current()
      }
      if (event.key === 'Tab') {
        const nodes = Array.from(
          ref.current?.querySelectorAll<HTMLElement>(
            'button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled),a[href],[tabindex="0"]',
          ) || [],
        )
        const first = nodes[0],
          last = nodes[nodes.length - 1]
        if (
          !ref.current?.contains(document.activeElement) ||
          document.activeElement === ref.current
        ) {
          event.preventDefault()
          ;(event.shiftKey ? last : first)?.focus()
        } else if (event.shiftKey && document.activeElement === first) {
          event.preventDefault()
          last?.focus()
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault()
          ;(first || ref.current)?.focus()
        }
      }
    }
    document.addEventListener('keydown', handler)
    return () => {
      document.removeEventListener('keydown', handler)
      previous?.focus()
    }
  }, [])
  return createPortal(
    <div
      className="modal-backdrop"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget && !locked) {
          onClose()
        }
      }}
    >
      <div
        className={'modal ' + (wide ? 'wide' : '')}
        role="dialog"
        aria-modal="true"
        tabIndex={-1}
        aria-labelledby={id}
        ref={ref}
      >
        <div className="modal-heading">
          <h2 id={id}>{title}</h2>
          <button
            className="icon-button"
            aria-label="Close dialog"
            onClick={onClose}
            disabled={locked}
          >
            <X size={20} />
          </button>
        </div>
        {children}
      </div>
    </div>,
    document.body,
  )
}
