import { useLayoutEffect, useRef, useState } from 'react'

export function useGraphFrame(expanded: boolean) {
  const frame = useRef<HTMLElement>(null)
  const [ready, setReady] = useState(false)
  useLayoutEffect(() => {
    const element = frame.current
    if (!element || expanded) return
    let scheduled = 0
    const measure = () => {
      const viewport = window.visualViewport?.height || window.innerHeight
      const top = element.getBoundingClientRect().top + window.scrollY
      const height = Math.max(300, viewport - top - 22) + 'px'
      if (element.style.getPropertyValue('--graph-height') !== height)
        element.style.setProperty('--graph-height', height)
    }
    const update = () => {
      cancelAnimationFrame(scheduled)
      scheduled = requestAnimationFrame(measure)
    }
    measure()
    setReady(true)
    const observer = new ResizeObserver(update)
    if (element.parentElement) observer.observe(element.parentElement)
    for (const target of element
      .closest('main')
      ?.querySelectorAll(
        '.page-title,.tabs,.section-heading,.graph-toolbar,.graph-footnote',
      ) || [])
      observer.observe(target)
    window.addEventListener('resize', update)
    window.visualViewport?.addEventListener('resize', update)
    update()
    return () => {
      cancelAnimationFrame(scheduled)
      observer.disconnect()
      window.removeEventListener('resize', update)
      window.visualViewport?.removeEventListener('resize', update)
    }
  }, [expanded])
  return { ref: frame, ready }
}
