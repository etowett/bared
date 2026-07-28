import { useId, useState } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import { Input } from '../ui/input'
import { Button } from '../ui/button'
import { FieldError, FieldHint } from './FormLayout'

interface PasswordInputProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  isEdit?: boolean
  label?: string
  required?: boolean
  /** Overrides the generated input id, e.g. to match an external `htmlFor`. */
  id?: string
  /** Inline validation message. Wires up `aria-invalid` and `aria-describedby`. */
  error?: string
  /** Set by a form that focuses the first invalid field on submit. */
  inputRef?: React.Ref<HTMLInputElement>
}

export function PasswordInput({
  value,
  onChange,
  placeholder = '',
  isEdit = false,
  label,
  required = false,
  id,
  error,
  inputRef,
}: PasswordInputProps) {
  const [showPassword, setShowPassword] = useState(false)
  const generatedId = useId()
  const inputId = id ?? generatedId
  const errorId = `${inputId}-error`
  const hintId = `${inputId}-hint`

  const displayPlaceholder = isEdit && !value ? 'Leave blank to keep existing value' : placeholder
  const showHint = isEdit && !value

  return (
    <div className="space-y-2">
      {label && (
        <label htmlFor={inputId} className="text-sm font-medium">
          {label}
          {required && <span className="text-danger ml-1">*</span>}
        </label>
      )}
      <div className="relative">
        <Input
          id={inputId}
          ref={inputRef}
          type={showPassword ? 'text' : 'password'}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={displayPlaceholder}
          required={required && !isEdit}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? errorId : showHint ? hintId : undefined}
          className="pr-10"
        />
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="absolute right-0 top-0 h-full px-3 hover:bg-transparent"
          onClick={() => setShowPassword(!showPassword)}
          // Icon-only, so it needs a name of its own; `title` is not one.
          aria-label={showPassword ? 'Hide password' : 'Show password'}
          tabIndex={-1}
        >
          {showPassword ? (
            <EyeOff aria-hidden="true" className="h-4 w-4 text-muted-foreground" />
          ) : (
            <Eye aria-hidden="true" className="h-4 w-4 text-muted-foreground" />
          )}
        </Button>
      </div>
      {error && <FieldError id={errorId}>{error}</FieldError>}
      {!error && showHint && (
        <FieldHint id={hintId}>Current value will be retained if left blank</FieldHint>
      )}
    </div>
  )
}
