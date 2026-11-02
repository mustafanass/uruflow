import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'

import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  Search,
  X,
} from 'lucide-react'

import { useTableState } from '@/shared/hooks/useTableState'

import { EmptyState } from './EmptyState'

interface Column<T> {
  key: string
  title: string
  render: (row: T) => ReactNode
  sort?: (row: T) => string | number
  className?: string
}
const pageSize = 50
export function DataTable<T>({
  scope,
  rows,
  columns,
  rowKey,
  onRow,
  rowHref,
  searchText,
  placeholder = 'Filter resources…',
  empty = 'No resources yet',
  emptyDetail,
  actions,
}: {
  scope: string
  rows: T[]
  columns: Column<T>[]
  rowKey: (row: T) => string
  onRow?: (row: T) => void
  rowHref?: (row: T) => string | undefined
  searchText?: (row: T) => string
  placeholder?: string
  empty?: string
  emptyDetail?: string
  actions?: ReactNode
}) {
  const { filter, sort, page, setFilter, setSort, setPage } =
    useTableState(scope)
  const query = searchText ? filter.trim().toLowerCase() : ''
  const filtered = query
    ? rows.filter((row) => searchText!(row).toLowerCase().includes(query))
    : [...rows]
  const sorter = columns.find((column) => column.key === sort.key)?.sort
  if (sorter) {
    filtered.sort((a, b) => {
      const av = sorter(a),
        bv = sorter(b)
      return (
        (typeof av === 'number' && typeof bv === 'number'
          ? av - bv
          : String(av).localeCompare(String(bv))) * (sort.desc ? -1 : 1)
      )
    })
  }
  const pageCount = Math.max(1, Math.ceil(filtered.length / pageSize))
  const currentPage = Math.min(page, pageCount)
  const start = (currentPage - 1) * pageSize
  const visible = filtered.slice(start, start + pageSize)
  return (
    <div className="data-table" data-table={scope}>
      {(searchText || actions) && (
        <div className="table-toolbar">
          {searchText && (
            <label className="filter-input">
              <Search size={16} />
              <input
                name="q"
                type="search"
                autoComplete="off"
                spellCheck={false}
                aria-label="Filter resources"
                placeholder={placeholder}
                maxLength={256}
                value={filter}
                onChange={(event) => setFilter(event.target.value)}
              />
              {filter && (
                <button
                  className="icon-button"
                  aria-label="Clear filter"
                  onClick={() => setFilter('')}
                >
                  <X size={14} />
                </button>
              )}
            </label>
          )}
          <div className="toolbar-actions">{actions}</div>
        </div>
      )}
      {filtered.length ? (
        <>
          <div
            className="table-scroll"
            tabIndex={0}
            role="region"
            aria-label="Resource table"
          >
            <table>
              <thead>
                <tr>
                  {columns.map((column) => (
                    <th
                      key={column.key}
                      className={column.className}
                      scope="col"
                      aria-sort={
                        sort.key === column.key
                          ? sort.desc
                            ? 'descending'
                            : 'ascending'
                          : undefined
                      }
                    >
                      {column.sort ? (
                        <button onClick={() => setSort(column.key)}>
                          {column.title}
                          {sort.key === column.key &&
                            (sort.desc ? (
                              <ChevronDown size={13} />
                            ) : (
                              <ChevronUp size={13} />
                            ))}
                        </button>
                      ) : (
                        column.title
                      )}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {visible.map((row) => (
                  <tr
                    key={rowKey(row)}
                    className={onRow || rowHref ? 'interactive' : ''}
                  >
                    {columns.map((column, index) => {
                      const content = column.render(row)
                      const href = index === 0 ? rowHref?.(row) : undefined
                      return (
                        <td key={column.key} className={column.className}>
                          {href ? (
                            <Link className="row-link" to={href}>
                              {content}
                            </Link>
                          ) : index === 0 && onRow ? (
                            <button
                              className="row-button"
                              onClick={() => onRow(row)}
                            >
                              {content}
                            </button>
                          ) : (
                            content
                          )}
                        </td>
                      )
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="table-footer">
            <span>
              {start + 1}–{Math.min(start + pageSize, filtered.length)} of{' '}
              {filtered.length}
              {query ? ' matches' : ''}
            </span>
            {pageCount > 1 && (
              <div className="pagination" aria-label="Table pages">
                <button
                  className="icon-button"
                  aria-label="Previous page"
                  disabled={currentPage === 1}
                  onClick={() => setPage(currentPage - 1)}
                >
                  <ChevronLeft size={16} />
                </button>
                <span>
                  {currentPage} / {pageCount}
                </span>
                <button
                  className="icon-button"
                  aria-label="Next page"
                  disabled={currentPage === pageCount}
                  onClick={() => setPage(currentPage + 1)}
                >
                  <ChevronRight size={16} />
                </button>
              </div>
            )}
          </div>
        </>
      ) : (
        <EmptyState
          title={query ? 'No matching resources' : empty}
          detail={query ? 'Try another filter.' : emptyDetail}
        />
      )}
    </div>
  )
}
