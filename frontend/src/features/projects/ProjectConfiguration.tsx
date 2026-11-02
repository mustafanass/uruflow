import { useState } from 'react'
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
  const [yamlChanged, setYamlChanged] = useState(false)
  const [secretsChanged, setSecretsChanged] = useState(false)
  const modified = [
    ...(yamlChanged ? [yaml] : []),
    ...(secretsChanged ? [secrets] : []),
  ]
  return (
    <section
      className="project-configuration"
      aria-label="Project configuration editors"
    >
      <Tabs
        items={[yaml, secrets]}
        value={tab}
        onChange={setTab}
        modified={modified}
      />
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
