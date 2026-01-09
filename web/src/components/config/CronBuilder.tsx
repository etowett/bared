import { useState } from 'react'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs'

interface CronBuilderProps {
  value: string
  onChange: (value: string) => void
  label?: string
  required?: boolean
}

export function CronBuilder({ value, onChange, label, required = false }: CronBuilderProps) {
  const [mode, setMode] = useState<'simple' | 'advanced'>('simple')

  // Parse cron expression for simple mode
  const parseCron = (cron: string) => {
    const parts = cron.split(' ')
    if (parts.length !== 5) {
      return { minute: '0', hour: '2', dayOfWeek: '*', preset: 'custom' }
    }

    const [minute, hour, , , dayOfWeek] = parts

    // Check for common presets
    if (cron === '0 2 * * *') return { minute, hour, dayOfWeek, preset: 'daily' }
    if (cron === '0 2 * * 0') return { minute, hour, dayOfWeek, preset: 'weekly' }
    if (cron === '0 2 1 * *') return { minute, hour, dayOfWeek, preset: 'monthly' }

    return { minute, hour, dayOfWeek, preset: 'custom' }
  }

  const [simpleMode, setSimpleMode] = useState(() => parseCron(value))

  const buildCron = (preset: string, minute: string, hour: string, dayOfWeek: string) => {
    switch (preset) {
      case 'hourly':
        return `${minute} * * * *`
      case 'daily':
        return `${minute} ${hour} * * *`
      case 'weekly':
        return `${minute} ${hour} * * ${dayOfWeek}`
      case 'monthly':
        return `${minute} ${hour} 1 * *`
      default:
        return value
    }
  }

  const handlePresetChange = (preset: string) => {
    setSimpleMode({ ...simpleMode, preset })
    if (preset !== 'custom') {
      const newCron = buildCron(preset, simpleMode.minute, simpleMode.hour, simpleMode.dayOfWeek)
      onChange(newCron)
    }
  }

  const handleTimeChange = (field: 'minute' | 'hour', newValue: string) => {
    const updated = { ...simpleMode, [field]: newValue }
    setSimpleMode(updated)
    const newCron = buildCron(updated.preset, updated.minute, updated.hour, updated.dayOfWeek)
    onChange(newCron)
  }

  const handleDayChange = (day: string) => {
    setSimpleMode({ ...simpleMode, dayOfWeek: day })
    const newCron = buildCron(simpleMode.preset, simpleMode.minute, simpleMode.hour, day)
    onChange(newCron)
  }

  const getDayName = (day: string) => {
    const days: Record<string, string> = {
      '0': 'Sunday',
      '1': 'Monday',
      '2': 'Tuesday',
      '3': 'Wednesday',
      '4': 'Thursday',
      '5': 'Friday',
      '6': 'Saturday',
    }
    return days[day] || 'Sunday'
  }

  return (
    <div className="space-y-2">
      {label && (
        <Label>
          {label}
          {required && <span className="text-red-500 ml-1">*</span>}
        </Label>
      )}

      <Tabs value={mode} onValueChange={(v) => setMode(v as any)} className="w-full">
        <TabsList className="grid w-full grid-cols-2">
          <TabsTrigger value="simple">Simple</TabsTrigger>
          <TabsTrigger value="advanced">Advanced</TabsTrigger>
        </TabsList>

        <TabsContent value="simple" className="space-y-4 mt-4">
          <div className="space-y-2">
            <Label htmlFor="preset">Frequency</Label>
            <Select value={simpleMode.preset} onValueChange={handlePresetChange}>
              <SelectTrigger id="preset">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="hourly">Hourly</SelectItem>
                <SelectItem value="daily">Daily</SelectItem>
                <SelectItem value="weekly">Weekly</SelectItem>
                <SelectItem value="monthly">Monthly</SelectItem>
                <SelectItem value="custom">Custom (use Advanced tab)</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {simpleMode.preset === 'hourly' && (
            <div className="space-y-2">
              <Label htmlFor="minute">At Minute</Label>
              <Input
                id="minute"
                type="number"
                min="0"
                max="59"
                value={simpleMode.minute}
                onChange={(e) => handleTimeChange('minute', e.target.value)}
              />
              <p className="text-xs text-gray-500">
                Run every hour at minute {simpleMode.minute}
              </p>
            </div>
          )}

          {(simpleMode.preset === 'daily' ||
            simpleMode.preset === 'weekly' ||
            simpleMode.preset === 'monthly') && (
            <>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="hour">Hour (24h)</Label>
                  <Input
                    id="hour"
                    type="number"
                    min="0"
                    max="23"
                    value={simpleMode.hour}
                    onChange={(e) => handleTimeChange('hour', e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="minute">Minute</Label>
                  <Input
                    id="minute"
                    type="number"
                    min="0"
                    max="59"
                    value={simpleMode.minute}
                    onChange={(e) => handleTimeChange('minute', e.target.value)}
                  />
                </div>
              </div>

              {simpleMode.preset === 'weekly' && (
                <div className="space-y-2">
                  <Label htmlFor="dayOfWeek">Day of Week</Label>
                  <Select
                    value={simpleMode.dayOfWeek}
                    onValueChange={handleDayChange}
                  >
                    <SelectTrigger id="dayOfWeek">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="0">Sunday</SelectItem>
                      <SelectItem value="1">Monday</SelectItem>
                      <SelectItem value="2">Tuesday</SelectItem>
                      <SelectItem value="3">Wednesday</SelectItem>
                      <SelectItem value="4">Thursday</SelectItem>
                      <SelectItem value="5">Friday</SelectItem>
                      <SelectItem value="6">Saturday</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              )}

              <p className="text-xs text-gray-500">
                {simpleMode.preset === 'daily' &&
                  `Run daily at ${simpleMode.hour.padStart(2, '0')}:${simpleMode.minute.padStart(2, '0')}`}
                {simpleMode.preset === 'weekly' &&
                  `Run every ${getDayName(simpleMode.dayOfWeek)} at ${simpleMode.hour.padStart(2, '0')}:${simpleMode.minute.padStart(2, '0')}`}
                {simpleMode.preset === 'monthly' &&
                  `Run on the 1st of every month at ${simpleMode.hour.padStart(2, '0')}:${simpleMode.minute.padStart(2, '0')}`}
              </p>
            </>
          )}

          {simpleMode.preset === 'custom' && (
            <p className="text-sm text-amber-600 dark:text-amber-400">
              Switch to Advanced tab to edit custom cron expression
            </p>
          )}
        </TabsContent>

        <TabsContent value="advanced" className="space-y-4 mt-4">
          <div className="space-y-2">
            <Label htmlFor="cron">Cron Expression</Label>
            <Input
              id="cron"
              value={value}
              onChange={(e) => onChange(e.target.value)}
              placeholder="0 2 * * *"
              required={required}
              className="font-mono"
            />
            <p className="text-xs text-gray-500">
              Format: minute hour day-of-month month day-of-week
            </p>
            <p className="text-xs text-gray-500">
              Example: <code className="bg-gray-100 dark:bg-gray-800 px-1 rounded">0 2 * * *</code> = Daily at 2:00 AM
            </p>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  )
}
