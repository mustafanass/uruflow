import { count as resourceCount } from '@/shared/utils'

import { type Edge, type ReactFlowInstance } from '@xyflow/react'

import {
  Expand,
  Maximize2,
  Minimize2,
  Focus,
  HardDrive,
  Minus,
  Plus,
  RotateCcw,
  Save,
  Search,
  X,
} from 'lucide-react'

import { type ResourceFlowNode } from './ResourceNode'

interface Props {
  query: string
  setQuery: (value: string) => void
  count: number
  attachments: boolean
  toggleAttachments: () => void
  focused: boolean
  toggleFocus: () => void
  hasSelection: boolean
  saveError: boolean
  save: () => void
  reset: () => void
  fit: () => void
  instance: ReactFlowInstance<ResourceFlowNode, Edge> | null
  maximized: boolean
  maximize: () => void
}

export function GraphToolbar(props: Props) {
  return (
    <div className="graph-toolbar">
      <label className="filter-input">
        <Search size={16} aria-hidden="true" />
        <input
          aria-label="Find graph resources"
          placeholder="Find a resource…"
          value={props.query}
          onChange={(event) => props.setQuery(event.target.value)}
          name="query"
          autoComplete="off"
          spellCheck={false}
        />
        {props.query && (
          <button
            className="icon-button"
            aria-label="Clear graph search"
            onClick={() => props.setQuery('')}
          >
            <X size={14} />
          </button>
        )}
      </label>
      <span className="graph-count">
        {resourceCount(props.count, 'resource')}
      </span>
      <div className="toolbar-actions">
        <button
          className="button small"
          aria-pressed={props.attachments}
          aria-label="Attachments"
          title="Network and storage attachments"
          onClick={props.toggleAttachments}
        >
          <HardDrive size={14} />
          <span className="graph-tool-label">Attachments</span>
        </button>
        <button
          className="button small"
          aria-pressed={props.focused}
          aria-label="Focus selected resource"
          title="Focus selected resource"
          disabled={!props.hasSelection}
          onClick={props.toggleFocus}
        >
          <Focus size={14} />
          <span className="graph-tool-label">Focus</span>
        </button>
        <div className="graph-zoom">
          <button
            className="icon-button"
            aria-label="Zoom in"
            onClick={() => props.instance?.zoomIn()}
          >
            <Plus size={17} />
          </button>
          <button
            className="icon-button"
            aria-label="Zoom out"
            onClick={() => props.instance?.zoomOut()}
          >
            <Minus size={17} />
          </button>
          <button
            className="icon-button"
            aria-label="Fit graph to view"
            onClick={props.fit}
          >
            <Expand size={17} />
          </button>
        </div>
      </div>
      <div className="graph-layout-actions">
        <button
          className="icon-button"
          aria-label="Reset layout"
          title="Reset layout"
          onClick={props.reset}
        >
          <RotateCcw size={16} />
        </button>
        <button
          className="icon-button"
          aria-label={props.maximized ? 'Exit expanded graph' : 'Expand graph'}
          aria-pressed={props.maximized}
          title={props.maximized ? 'Exit expanded graph' : 'Expand graph'}
          onClick={props.maximize}
        >
          {props.maximized ? (
            <Minimize2 size={16} aria-hidden="true" />
          ) : (
            <Maximize2 size={16} aria-hidden="true" />
          )}
          <span className="graph-expand-label">
            {props.maximized ? 'Exit' : 'Expand'}
          </span>
        </button>
        {props.saveError && (
          <button className="button primary small" onClick={props.save}>
            <Save size={14} />
            Retry save
          </button>
        )}
      </div>
    </div>
  )
}
