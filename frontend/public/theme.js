try {
  const theme = localStorage.getItem('uruflow:theme')
  document.documentElement.dataset.theme = ['light', 'dark'].includes(theme)
    ? theme
    : 'system'
} catch {
  document.documentElement.dataset.theme = 'system'
}

window.addEventListener('storage', (event) => {
  if (event.key !== 'uruflow:theme' && event.key !== null) return
  const theme = event.newValue
  document.documentElement.dataset.theme = ['light', 'dark'].includes(theme)
    ? theme
    : 'system'
  window.dispatchEvent(new Event('uruflow:theme'))
})
