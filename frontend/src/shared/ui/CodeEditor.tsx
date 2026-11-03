import { yaml } from '@codemirror/lang-yaml'
import CodeMirror from '@uiw/react-codemirror'
import { useTheme } from '@/shared/hooks/useTheme'

export default function CodeEditor({
  value,
  onChange,
  variables = false,
  readOnly = false,
  height = '440px',
}: {
  value: string
  onChange: (value: string) => void
  variables?: boolean
  readOnly?: boolean
  height?: string
}) {
  const { resolved } = useTheme()
  return (
    <CodeMirror
      className="code-editor"
      value={value}
      height={height}
      width="100%"
      theme={resolved === 'dark' ? 'dark' : 'light'}
      extensions={variables ? [] : [yaml()]}
      onChange={onChange}
      editable={!readOnly}
      aria-label={
        variables ? 'Project variables editor' : 'Project YAML editor'
      }
      basicSetup={{
        lineNumbers: true,
        foldGutter: !variables,
        highlightActiveLine: true,
        highlightSelectionMatches: true,
        autocompletion: false,
      }}
    />
  )
}
