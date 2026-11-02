import { useRef, type ReactNode } from 'react'

const tabID = (value: string) => 'page-tab-' + encodeURIComponent(value)
const panelID = (value: string) => 'page-panel-' + encodeURIComponent(value)

export function Tabs({
  items,
  value,
  onChange,
  modified = [],
}: {
  items: string[]
  value: string
  onChange: (value: string) => void
  modified?: readonly string[]
}) {
  const root = useRef<HTMLDivElement>(null)
  return (
    <div className="tabs" role="tablist" aria-label="Page sections" ref={root}>
      {items.map((item, index) => (
        <button
          type="button"
          key={item}
          role="tab"
          id={tabID(item)}
          aria-controls={panelID(item)}
          aria-label={
            modified.includes(item) ? item + ', unsaved changes' : undefined
          }
          aria-selected={item === value}
          tabIndex={item === value ? 0 : -1}
          className={item === value ? 'selected' : ''}
          onClick={() => onChange(item)}
          onKeyDown={(event) => {
            let next = index
            if (event.key === 'ArrowRight') {
              next = (index + 1) % items.length
            } else if (event.key === 'ArrowLeft') {
              next = (index - 1 + items.length) % items.length
            } else if (event.key === 'Home') {
              next = 0
            } else if (event.key === 'End') {
              next = items.length - 1
            } else {
              return
            }
            event.preventDefault()
            onChange(items[next])
            root.current
              ?.querySelectorAll<HTMLButtonElement>('[role=tab]')
              [next]?.focus()
          }}
        >
          {item}
          {modified.includes(item) && (
            <span className="tab-modified" aria-hidden="true" />
          )}
        </button>
      ))}
    </div>
  )
}

export function TabPanel({
  value,
  children,
  hidden = false,
}: {
  value: string
  children: ReactNode
  hidden?: boolean
}) {
  return (
    <section
      hidden={hidden}
      role="tabpanel"
      id={panelID(value)}
      aria-labelledby={tabID(value)}
      tabIndex={0}
    >
      {children}
    </section>
  )
}
