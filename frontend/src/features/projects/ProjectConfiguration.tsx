import { useEffect, useState } from 'react'
import { Maximize2, Minimize2 } from 'lucide-react'
import type { Project } from '@/shared/types'
import { Tabs, TabPanel } from '@/shared/ui'
import { ProjectEditor } from './ProjectEditor'

const yaml = 'YAML'
const secrets = 'Secrets & variables'

export function ProjectConfiguration({
  project,
  initialSecrets = false,
}: {
  project: Project
  initialSecrets?: boolean
}) {
  const [tab, setTab] = useState(initialSecrets ? secrets : yaml)
  const [expanded, setExpanded] = useState(false)
  useEffect(() => {
    if (!expanded) return
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const close = (event: KeyboardEvent) => {
      if (
        event.key === 'Escape' &&
        !event.defaultPrevented &&
        !document.querySelector('[aria-modal="true"]')
      )
        setExpanded(false)
    }
    document.addEventListener('keydown', close)
    return () => {
      document.body.style.overflow = previous
      document.removeEventListener('keydown', close)
    }
  }, [expanded])
  const [yamlChanged, setYamlChanged] = useState(false)
  const [secretsChanged, setSecretsChanged] = useState(false)
  const modified = [
    ...(yamlChanged ? [yaml] : []),
    ...(secretsChanged ? [secrets] : []),
  ]
  return (
    <section
      className={'project-configuration' + (expanded ? ' is-expanded' : '')}
      aria-label="Project configuration editors"
    >
      <div className="configuration-toolbar">
        <Tabs
          items={[yaml, secrets]}
          value={tab}
          onChange={setTab}
          modified={modified}
        />
        <button
          className="icon-button"
          aria-label={expanded ? 'Exit expanded editor' : 'Expand editor'}
          title={expanded ? 'Exit expanded editor' : 'Expand editor'}
          aria-pressed={expanded}
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? (
            <Minimize2 size={17} aria-hidden="true" />
          ) : (
            <Maximize2 size={17} aria-hidden="true" />
          )}
        </button>
      </div>
      <TabPanel value={yaml} hidden={tab !== yaml}>
        <ProjectEditor
          project={project}
          variables={false}
          onDirtyChange={setYamlChanged}
        />
      </TabPanel>
      <TabPanel value={secrets} hidden={tab !== secrets}>
        <ProjectEditor
          project={project}
          variables
          onDirtyChange={setSecretsChanged}
        />
      </TabPanel>
    </section>
  )
}
