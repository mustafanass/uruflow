import { useSearchParams } from 'react-router-dom'

export function useTableState(scope: string) {
  const parameter = (name: string) => scope + '.' + name
  const [params, setParams] = useSearchParams()
  function update(values: Record<string, string>) {
    setParams(
      (current) => {
        const next = new URLSearchParams(current)
        for (const [name, value] of Object.entries(values)) {
          if (value) {
            next.set(parameter(name), value)
          } else {
            next.delete(parameter(name))
          }
        }
        return next
      },
      { replace: true, preventScrollReset: true },
    )
  }
  const requestedPage = Number(params.get(parameter('page')) || 1)
  return {
    filter: params.get(parameter('q')) || '',
    sort: {
      key: params.get(parameter('sort')) || '',
      desc: params.get(parameter('order')) === 'desc',
    },
    page:
      Number.isSafeInteger(requestedPage) && requestedPage > 0
        ? requestedPage
        : 1,
    setFilter: (value: string) => update({ q: value, page: '' }),
    setSort: (key: string) =>
      update({
        sort: key,
        order:
          params.get(parameter('sort')) === key &&
          params.get(parameter('order')) !== 'desc'
            ? 'desc'
            : 'asc',
        page: '',
      }),
    setPage: (value: number) =>
      update({ page: value === 1 ? '' : String(value) }),
  }
}
