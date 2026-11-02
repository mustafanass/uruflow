import React from 'react'
import ReactDOM from 'react-dom/client'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'

import '@fontsource-variable/public-sans'
import '@fontsource/ibm-plex-mono/400.css'
import '@xyflow/react/dist/style.css'

import { App } from './app/App'
import { PageError } from './app/PageError'
import './styles/index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <RouterProvider
      router={createBrowserRouter([
        { path: '*', element: <App />, errorElement: <PageError /> },
      ])}
    />
  </React.StrictMode>,
)
