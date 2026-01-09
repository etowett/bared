import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '../api/client'
import type {
  Storage,
  StorageRequest,
  Notifier,
  NotifierRequest,
  TargetConfig,
  TargetConfigRequest,
  RestoreTargetConfig,
  RestoreTargetConfigRequest,
  GlobalConfig,
} from '../types'

// Storages
export function useStorages() {
  return useQuery({
    queryKey: ['config', 'storages'],
    queryFn: () => apiClient.getStorages(),
  })
}

export function useStorage(name: string) {
  return useQuery({
    queryKey: ['config', 'storages', name],
    queryFn: () => apiClient.getStorage(name),
    enabled: !!name,
  })
}

export function useCreateStorage() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (storage: StorageRequest) => apiClient.createStorage(storage),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config', 'storages'] })
    },
  })
}

export function useUpdateStorage() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ name, storage }: { name: string; storage: StorageRequest }) =>
      apiClient.updateStorage(name, storage),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['config', 'storages'] })
      queryClient.invalidateQueries({ queryKey: ['config', 'storages', variables.name] })
    },
  })
}

export function useDeleteStorage() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => apiClient.deleteStorage(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config', 'storages'] })
    },
  })
}

// Notifiers
export function useNotifiers() {
  return useQuery({
    queryKey: ['config', 'notifiers'],
    queryFn: () => apiClient.getNotifiers(),
  })
}

export function useCreateNotifier() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (notifier: NotifierRequest) => apiClient.createNotifier(notifier),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config', 'notifiers'] })
    },
  })
}

export function useUpdateNotifier() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ name, notifier }: { name: string; notifier: NotifierRequest }) =>
      apiClient.updateNotifier(name, notifier),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config', 'notifiers'] })
    },
  })
}

export function useDeleteNotifier() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => apiClient.deleteNotifier(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config', 'notifiers'] })
    },
  })
}

// Targets Config
export function useTargetsConfig() {
  return useQuery({
    queryKey: ['config', 'targets'],
    queryFn: () => apiClient.getTargetsConfig(),
  })
}

export function useCreateTargetConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (target: TargetConfigRequest) => apiClient.createTargetConfig(target),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config', 'targets'] })
      queryClient.invalidateQueries({ queryKey: ['targets'] })
    },
  })
}

export function useUpdateTargetConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ name, target }: { name: string; target: TargetConfigRequest }) =>
      apiClient.updateTargetConfig(name, target),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config', 'targets'] })
      queryClient.invalidateQueries({ queryKey: ['targets'] })
    },
  })
}

export function useUpdateTargetSchedule() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ name, schedule }: { name: string; schedule: string }) =>
      apiClient.updateTargetSchedule(name, schedule),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config', 'targets'] })
      queryClient.invalidateQueries({ queryKey: ['targets'] })
    },
  })
}

export function useDeleteTargetConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => apiClient.deleteTargetConfig(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config', 'targets'] })
      queryClient.invalidateQueries({ queryKey: ['targets'] })
    },
  })
}

// Restore Targets Config
export function useRestoreTargetsConfig() {
  return useQuery({
    queryKey: ['config', 'restore-targets'],
    queryFn: () => apiClient.getRestoreTargetsConfig(),
  })
}

export function useCreateRestoreTargetConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (target: RestoreTargetConfigRequest) => apiClient.createRestoreTargetConfig(target),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config', 'restore-targets'] })
      queryClient.invalidateQueries({ queryKey: ['restore-targets'] })
    },
  })
}

export function useUpdateRestoreTargetConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ name, target }: { name: string; target: RestoreTargetConfigRequest }) =>
      apiClient.updateRestoreTargetConfig(name, target),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config', 'restore-targets'] })
      queryClient.invalidateQueries({ queryKey: ['restore-targets'] })
    },
  })
}

export function useDeleteRestoreTargetConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => apiClient.deleteRestoreTargetConfig(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config', 'restore-targets'] })
      queryClient.invalidateQueries({ queryKey: ['restore-targets'] })
    },
  })
}

// Global Config
export function useGlobalConfig() {
  return useQuery({
    queryKey: ['config', 'global'],
    queryFn: () => apiClient.getGlobalConfig(),
  })
}

export function useUpdateGlobalConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) =>
      apiClient.updateGlobalConfig(key, value),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config', 'global'] })
    },
  })
}

// Config Utilities
export function useConfigSource() {
  return useQuery({
    queryKey: ['config', 'source'],
    queryFn: () => apiClient.getConfigSource(),
  })
}

export function useMigrateConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => apiClient.migrateConfig(),
    onSuccess: () => {
      // Invalidate all config queries
      queryClient.invalidateQueries({ queryKey: ['config'] })
    },
  })
}

export function useReloadConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => apiClient.reloadConfig(),
    onSuccess: () => {
      // Invalidate all config and runtime data queries
      queryClient.invalidateQueries({ queryKey: ['config'] })
      queryClient.invalidateQueries({ queryKey: ['targets'] })
      queryClient.invalidateQueries({ queryKey: ['restore-targets'] })
    },
  })
}
