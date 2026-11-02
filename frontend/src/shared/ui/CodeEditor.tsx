import { yaml } from '@codemirror/lang-yaml'
import CodeMirror from '@uiw/react-codemirror'
import { useTheme } from '@/shared/hooks/useTheme'

export default function CodeEditor({
  value,
  onChange,
  variables = false,
  readOnly = false,
}: {
  value: string
  onChange: (value: string) => void
  variables?: boolean
  readOnly?: boolean
}) {
  const { resolved } = useTheme()
  return (
    <CodeMirror
      value={value}
      height="440px"
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
