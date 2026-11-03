import { useEffect, useId, useRef, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'

import { Search } from 'lucide-react'

import { segment } from '@/shared/api'
import { useWorkspace } from '@/shared/hooks'

export function ResourceSearch() {
  const { snapshot } = useWorkspace()
  const navigate = useNavigate()
  const location = useLocation()
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)
  const [open, setOpen] = useState(false)
  const input = useRef<HTMLInputElement>(null)
  const listId = useId()
  const needle = query.trim().toLowerCase()
  const results =
    !needle || !snapshot
      ? []
      : [
          ...snapshot.projects.map((p) => ({
            name: p.name,
            kind: 'Project',
            url: '/projects/' + segment(p.name),
          })),
          ...snapshot.agents.map((a) => ({
            name: a.name,
            kind: 'Agent',
            url: '/agents/' + segment(a.id),
          })),
          ...snapshot.containers.map((c) => ({
            name: c.name,
            kind: 'Container',
            url: '/containers/' + segment(c.agent_id) + '/' + segment(c.id),
          })),
          ...snapshot.releases.map((r) => ({
            name: r.project + ' / ' + r.id,
            kind: 'Release',
            url: '/releases/' + segment(r.id),
          })),
        ]
          .filter((item) =>
            (item.name + ' ' + item.kind).toLowerCase().includes(needle),
          )
          .slice(0, 8)
  useEffect(() => {
    setQuery('')
    setOpen(false)
    setActive(0)
  }, [location.pathname])
  useEffect(() => {
    const search = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key === 'k') {
        event.preventDefault()
        input.current?.focus()
      }
    }
    window.addEventListener('keydown', search)
    return () => window.removeEventListener('keydown', search)
  }, [])
  function select(index: number) {
    if (!results[index]) {
      return
    }
    navigate(results[index].url)
    setOpen(false)
    setQuery('')
  }
  return (
    <div
      className="global-search"
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) {
          setOpen(false)
        }
      }}
    >
      <Search size={16} aria-hidden="true" />
      <input
        ref={input}
        role="combobox"
        aria-label="Search all resources"
        aria-autocomplete="list"
        aria-expanded={open && !!needle}
        aria-controls={listId}
        aria-activedescendant={
          open && results[active] ? listId + '-' + active : undefined
        }
        placeholder="Search resources…"
        value={query}
        onFocus={() => setOpen(true)}
        onChange={(event) => {
          setQuery(event.target.value)
          setActive(0)
          setOpen(true)
        }}
        onKeyDown={(event) => {
          if (event.key === 'Escape') {
            setOpen(false)
            setQuery('')
          }
          if (event.key === 'ArrowDown') {
            event.preventDefault()
            setOpen(true)
            setActive((index) =>
              Math.min(index + 1, Math.max(0, results.length - 1)),
            )
          }
          if (event.key === 'ArrowUp') {
            event.preventDefault()
            setActive((index) => Math.max(index - 1, 0))
          }
          if (event.key === 'Enter') {
            event.preventDefault()
            select(Math.max(active, 0))
          }
        }}
        name="query"
        autoComplete="off"
        spellCheck={false}
      />
      {open && !!needle && (
        <div
          id={listId}
          className="search-results"
          role="listbox"
          aria-label="Search results"
        >
          {results.length ? (
            results.map((item, index) => (
              <button
                key={item.url}
                id={listId + '-' + index}
                role="option"
                aria-selected={index === active}
                className={index === active ? 'selected' : ''}
                onMouseEnter={() => setActive(index)}
                onClick={() => select(index)}
              >
                <span>{item.name}</span>
                <small>{item.kind}</small>
              </button>
            ))
          ) : (
            <p>No matching resources</p>
          )}
        </div>
      )}
    </div>
  )
}
