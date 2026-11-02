import type { Project } from '../../shared/types/project.ts'

export function releaseDefinition(spec?: Project) {
  if (!spec) return '{}\n'
  const definition = Object.fromEntries(
    Object.entries(spec).filter(
      ([key]) => key !== 'created_at' && key !== 'source',
    ),
  )
  const ordered = (value: unknown): unknown => {
    if (Array.isArray(value)) return value.map(ordered)
    if (value && typeof value === 'object')
      return Object.fromEntries(
        Object.entries(value)
          .sort(([a], [b]) => a.localeCompare(b))
          .map(([key, item]) => [key, ordered(item)]),
      )
    return value
  }
  return JSON.stringify(ordered(definition), null, 2) + '\n'
}
