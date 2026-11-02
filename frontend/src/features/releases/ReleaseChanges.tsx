import { Link } from 'react-router-dom'
import { diffLines } from 'diff'
import { segment } from '@/shared/api'
import { useLoad } from '@/shared/hooks'
import type { Release } from '@/shared/types'
import { CopyButton, EmptyState, ErrorState, Panel, Spinner } from '@/shared/ui'
import { short } from '@/shared/utils'
import { releaseDefinition } from './release-definition'

export function ReleaseChanges({ release }: { release: Release }) {
  const history = useLoad<Release[]>(
    '/api/projects/' + segment(release.project) + '/releases',
  )
  const index =
    history.data?.findIndex((value) => value.id === release.id) ?? -1
  const previousID = index >= 0 ? history.data?.[index + 1]?.id : undefined
  const previous = useLoad<Release>(
    previousID ? '/api/releases/' + segment(previousID) : '',
    !!previousID,
  )
  if (history.error)
    return <ErrorState message={history.error} retry={history.reload} />
  if (previous.error)
    return <ErrorState message={previous.error} retry={previous.reload} />
  if (!release.spec?.name)
    return (
      <EmptyState
        title="No saved definition"
        detail="This release does not contain a recorded project definition."
      />
    )
  if (!history.data || (previousID && !previous.data))
    return <Spinner label="Comparing release definitions…" />
  if (index < 0)
    return (
      <ErrorState
        message="The release history changed. Reload it to compare this release."
        retry={history.reload}
      />
    )
  const current = releaseDefinition(release.spec)
  const before = previousID ? releaseDefinition(previous.data?.spec) : ''
  const changed = current !== before
  const comparable = !previousID || !!previous.data?.spec?.name
  return (
    <>
      <Panel
        title="Configuration changes"
        detail={
          previousID ? (
            <>
              {comparable ? 'Compared with' : 'Previous release'}{' '}
              <Link to={'/releases/' + segment(previousID)}>
                {short(previousID)}
              </Link>
            </>
          ) : (
            'First release definition'
          )
        }
      >
        {!comparable ? (
          <p className="panel-note">
            The previous release has no saved definition to compare.
          </p>
        ) : changed ? (
          <pre className="diff-view">
            {diffLines(before, current).map((part, index) => (
              <span
                className={part.added ? 'added' : part.removed ? 'removed' : ''}
                key={index}
              >
                {part.value.split('\n').map((line, row) =>
                  line ? (
                    <span key={row}>
                      {part.added ? '+ ' : part.removed ? '− ' : '  '}
                      {line}
                      {'\n'}
                    </span>
                  ) : null,
                )}
              </span>
            ))}
          </pre>
        ) : (
          <p className="panel-note">No configuration changes.</p>
        )}
      </Panel>
      <details className="details-panel">
        <summary>Saved release definition</summary>
        <Panel
          actions={
            <CopyButton value={current} label="Copy release definition" />
          }
        >
          <pre className="json-view">{current}</pre>
        </Panel>
      </details>
    </>
  )
}
