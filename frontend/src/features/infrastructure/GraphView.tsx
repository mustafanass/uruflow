import { RelativeTime } from '@/shared/ui/RelativeTime'
import { useCallback, useEffect, useMemo, useState } from 'react'

import { type Edge, type ReactFlowInstance } from '@xyflow/react'

import { useWorkspace } from '@/shared/hooks'
import { type Graph, type Project } from '@/shared/types'
import { Spinner } from '@/shared/ui'
import { errorText } from '@/shared/utils'

import { filterGraph } from './filter-graph'
import { GraphCanvas } from './GraphCanvas'
import { GraphToolbar } from './GraphToolbar'
import { ResourceInspector } from './ResourceInspector'
import { type ResourceFlowNode } from './ResourceNode'
import { useGraphLayout } from './useGraphLayout'
import { fitGraph } from './graph-options'
import { ResourceLive } from './ResourceLive'
import { useGraphFrame } from './useGraphFrame'

interface GraphProps {
  graph: Graph
  scope: string
  project?: Project
}

export function GraphView(props: GraphProps) {
  const { session, snapshot, connection, stale } = useWorkspace()
  const storageKey =
    'uruflow:graph:v1:' + JSON.stringify([session.user?.username, props.scope])
  if (!snapshot) return <Spinner />
  return (
    <ResourceLive snapshot={snapshot} live={connection === 'live' && !stale}>
      <GraphWorkspace key={storageKey} {...props} storageKey={storageKey} />
    </ResourceLive>
  )
}

function GraphWorkspace({
  graph,
  project,
  storageKey,
}: GraphProps & { storageKey: string }) {
  const { snapshot, notify, confirm, connection, stale } = useWorkspace()
  const [maximized, setMaximized] = useState(false)
  const { ref: frame, ready } = useGraphFrame(maximized)
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const toggle = useCallback(
    (id: string) =>
      setExpanded((previous) => {
        const next = new Set(previous)
        if (next.has(id)) next.delete(id)
        else next.add(id)
        return next
      }),
    [],
  )
  const collapsed = useMemo(
    () =>
      new Set(
        graph.nodes
          .filter((node) => node.kind === 'agent' && !expanded.has(node.id))
          .map((node) => node.id),
      ),
    [graph.nodes, expanded],
  )
  const [query, setQuery] = useState('')
  const [attachments, setAttachments] = useState(false)
  const [focused, setFocused] = useState(false)
  const [selected, setSelected] = useState<string | null>(null)
  const [hovered, setHovered] = useState<string | null>(null)
  const [hoveredEdge, setHoveredEdge] = useState<string | null>(null)
  const [allConnections, setAllConnections] = useState(false)
  const [layoutError, setLayoutError] = useState(false)
  const [instance, setInstance] = useState<ReactFlowInstance<
    ResourceFlowNode,
    Edge
  > | null>(null)
  const current = graph.nodes.find((node) => node.id === selected) || null
  const base = useMemo(
    () => filterGraph(graph, project, '', attachments, false, null, collapsed),
    [graph, project, attachments, collapsed],
  )
  const view = useMemo(
    () =>
      filterGraph(
        graph,
        project,
        query,
        attachments,
        focused,
        current,
        collapsed,
      ),
    [graph, project, query, attachments, focused, current?.id, collapsed],
  )
  const {
    nodes,
    edges,
    onNodesChange,
    dirty,
    save,
    reset,
    customized,
    settled,
  } = useGraphLayout({
    graph,
    base,
    view,
    storageKey,
    selected,
    select: setSelected,
    projectName: project?.name,
    expanded,
    toggle,
    togglable: !query && !focused,
    filtered: !!query || focused,
    hovered,
    hoveredEdge,
    allConnections,
    live: connection === 'live' && !stale,
  })
  const closeInspector = useCallback(() => {
    setSelected(null)
    setFocused(false)
  }, [])

  useEffect(() => {
    if (!dirty || layoutError) return
    const timer = window.setTimeout(() => {
      try {
        save()
      } catch {
        setLayoutError(true)
      }
    }, 450)
    return () => clearTimeout(timer)
  }, [dirty, nodes, save, layoutError])

  useEffect(() => {
    if (!selected && !maximized) {
      return
    }
    const close = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        if (selected) closeInspector()
        else setMaximized(false)
      }
    }
    document.addEventListener('keydown', close)
    return () => document.removeEventListener('keydown', close)
  }, [selected, closeInspector, maximized])
  useEffect(() => {
    if (selected && !current) {
      closeInspector()
    }
  }, [selected, current, closeInspector])

  useEffect(() => {
    if (!maximized) return
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previous
    }
  }, [maximized])

  function saveLayout() {
    try {
      save()
      setLayoutError(false)
    } catch {
      notify(
        'Layout could not be saved. Allow browser storage and try again.',
        'error',
      )
    }
  }

  function resetLayout() {
    const run = async () => {
      reset()
      setLayoutError(false)
      closeInspector()
      requestAnimationFrame(fit)
      if (!dirty) notify('Automatic layout restored')
    }

    if (dirty) {
      confirm({
        title: 'Reset graph layout?',
        description:
          'Your moved nodes will return to their automatic positions.',
        label: 'Reset layout',
        success: 'Automatic layout restored',
        run,
      })
    } else {
      void run().catch((error) => notify(errorText(error), 'error'))
    }
  }

  function fit() {
    void fitGraph(instance, frame.current?.querySelector('.graph-canvas'))
  }

  if (!snapshot) {
    return <Spinner />
  }

  return (
    <section
      className={'graph-workspace' + (maximized ? ' is-expanded' : '')}
      aria-label="Infrastructure graph"
      ref={frame}
    >
      <GraphToolbar
        query={query}
        setQuery={setQuery}
        count={view.nodes.length}
        attachments={attachments}
        toggleAttachments={() => setAttachments((value) => !value)}
        focused={focused}
        toggleFocus={() => setFocused((value) => !value)}
        hasSelection={!!current}
        allConnections={allConnections}
        toggleConnections={() => setAllConnections((value) => !value)}
        saveError={layoutError}
        save={saveLayout}
        reset={resetLayout}
        fit={fit}
        instance={instance}
        maximized={maximized}
        maximize={() => setMaximized((value) => !value)}
      />
      <div className="graph-body">
        {ready && (
          <GraphCanvas
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            select={setSelected}
            close={closeInspector}
            instance={instance}
            initialize={setInstance}
            settled={settled}
            hoverNode={setHovered}
            hoverEdge={setHoveredEdge}
            allConnections={allConnections}
            fitKey={JSON.stringify([
              query,
              attachments,
              focused,
              maximized,
              [...expanded].sort(),
            ])}
          />
        )}
        {current && (
          <ResourceInspector
            node={current}
            project={project}
            close={closeInspector}
          />
        )}
      </div>
      <div className="graph-footnote">
        <span
          className={
            'status-dot ' + (connection !== 'live' || stale ? 'inactive' : '')
          }
        />
        <span>
          {connection === 'live' && !stale
            ? 'Live topology'
            : 'Last observed topology'}
        </span>
        <span className="graph-layout-status" role="status">
          {layoutError
            ? 'Layout not saved'
            : dirty
              ? 'Saving layout…'
              : customized
                ? 'Layout saved in this browser'
                : 'Automatic layout'}
        </span>
        <span>
          Updated <RelativeTime value={snapshot.at} />
        </span>
      </div>
    </section>
  )
}
