import { useState } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import { Input } from '../ui/input'
import { Button } from '../ui/button'

interface PasswordInputProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  isEdit?: boolean
  label?: string
  required?: boolean
}

export function PasswordInput({
  value,
  onChange,
  placeholder = '',
  isEdit = false,
  label,
  required = false,
}: PasswordInputProps) {
  const [showPassword, setShowPassword] = useState(false)

  const displayPlaceholder = isEdit && !value ? 'Leave blank to keep existing value' : placeholder

  return (
    <div className="space-y-2">
      {label && (
        <label className="text-sm font-medium">
          {label}
          {required && <span className="text-red-500 ml-1">*</span>}
        </label>
      )}
      <div className="relative">
        <Input
          type={showPassword ? 'text' : 'password'}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={displayPlaceholder}
          required={required && !isEdit}
          className="pr-10"
        />
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="absolute right-0 top-0 h-full px-3 hover:bg-transparent"
          onClick={() => setShowPassword(!showPassword)}
          tabIndex={-1}
        >
          {showPassword ? (
            <EyeOff className="h-4 w-4 text-gray-500" />
          ) : (
            <Eye className="h-4 w-4 text-gray-500" />
          )}
        </Button>
      </div>
      {isEdit && !value && (
        <p className="text-xs text-gray-500">Current value will be retained if left blank</p>
      )}
    </div>
  )
}
