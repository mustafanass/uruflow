import { Modal } from '@/shared/ui'

export function WorkspaceHelp({ close }: { close: () => void }) {
  return (
    <Modal title="Workspace help" onClose={close}>
      <div className="help-content">
        <p>
          Open a project to explore its infrastructure. Select a node for
          details.
        </p>
        <p>
          Sample age shows how recent the data is. Grey states mean unavailable
          or stale observations.
        </p>
        <p>
          <kbd>Ctrl/⌘ K</kbd> Search resources <span className="keyboard-gap" />
          <kbd>Esc</kbd> Close a dialog
        </p>
      </div>
      <div className="modal-actions">
        <button className="button primary" onClick={close}>
          Got it
        </button>
      </div>
    </Modal>
  )
}
