/* eslint-disable react-refresh/only-export-components */
import React, { ReactElement } from 'react'
import { render, RenderOptions } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider } from '../contexts/ThemeContext'

// Create a fresh QueryClient for each test
const createTestQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

interface CustomRenderOptions extends Omit<RenderOptions, 'wrapper'> {
  queryClient?: QueryClient
}

export function renderWithQuery(
  ui: ReactElement,
  { queryClient = createTestQueryClient(), ...renderOptions }: CustomRenderOptions = {}
) {
  const Wrapper = ({ children }: { children: React.ReactNode }) => (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </ThemeProvider>
  )
  return {
    ...render(ui, { wrapper: Wrapper, ...renderOptions }),
    queryClient,
  }
}

export * from '@testing-library/react'
export { renderWithQuery as render }
