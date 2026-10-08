import { useState, useEffect, useCallback } from 'react'

type Theme = 'dark' | 'light'

function readInitialTheme(): Theme {
  if (typeof window === 'undefined') return 'dark'
  return (document.documentElement.getAttribute('data-theme') as Theme) || 'dark'
}

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(readInitialTheme)

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    // Keep the browser status bar in step with the in-app toggle, since the
    // page is always dark by default regardless of the OS preference.
    const meta = document.querySelector('meta[name="theme-color"]:not([media])')
    meta?.setAttribute('content', theme === 'dark' ? '#08080a' : '#f4f1ea')
  }, [theme])

  const toggle = useCallback(() => {
    setTheme(prev => (prev === 'dark' ? 'light' : 'dark'))
  }, [])

  return { theme, toggle }
}
