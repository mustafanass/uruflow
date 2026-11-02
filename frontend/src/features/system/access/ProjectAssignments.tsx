import { useWorkspace } from '@/shared/hooks'

export function ProjectAssignments({
  value,
  onChange,
}: {
  value: string[]
  onChange: (projects: string[]) => void
}) {
  const { snapshot } = useWorkspace()
  const all = value.includes('*')
  const names = Array.from(
    new Set([
      ...(snapshot?.projects.map((p) => p.name) || []),
      ...value.filter((p) => p !== '*'),
    ]),
  ).sort()
  return (
    <fieldset className="project-assignments">
      <legend>Project access</legend>
      <label className="checkbox-label">
        <input
          type="checkbox"
          checked={all}
          onChange={(e) => onChange(e.target.checked ? ['*'] : [])}
          autoComplete="off"
          spellCheck={false}
        />
        All projects, including future projects
      </label>
      <p className="muted">
        A project grant includes its workloads, logs and participating agents’
        host metrics. Unassigned projects stay hidden.
      </p>
      {!all &&
        names.map((name) => (
          <label className="checkbox-label" key={name}>
            <input
              type="checkbox"
              checked={value.includes(name)}
              onChange={(e) =>
                onChange(
                  e.target.checked
                    ? [...value, name]
                    : value.filter((p) => p !== name),
                )
              }
              autoComplete="off"
              spellCheck={false}
            />
            {name}
          </label>
        ))}
      {!all && !names.length && (
        <p>
          No projects exist yet. This user will start without project access.
        </p>
      )}
    </fieldset>
  )
}
