import { useState, type InputHTMLAttributes } from 'react'

import { Eye, EyeOff } from 'lucide-react'

export function PasswordInput(props: InputHTMLAttributes<HTMLInputElement>) {
  const [visible, setVisible] = useState(false)
  return (
    <span className="password-field">
      <input
        {...props}
        type={visible ? 'text' : 'password'}
        spellCheck={false}
        autoComplete={props.autoComplete || 'current-password'}
      />
      <button
        type="button"
        className="icon-button"
        aria-label={visible ? 'Hide password' : 'Show password'}
        aria-pressed={visible}
        disabled={props.disabled}
        onClick={() => setVisible((value) => !value)}
      >
        {visible ? <EyeOff size={17} /> : <Eye size={17} />}
      </button>
    </span>
  )
}
