export function withTab(
  current: URLSearchParams,
  value: string,
  defaultTab?: string,
) {
  const next = new URLSearchParams(current)
  if (value === defaultTab) next.delete('tab')
  else next.set('tab', value)
  return next
}
