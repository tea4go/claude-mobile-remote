import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App.tsx'
import { Capacitor } from '@capacitor/core'
import { createCapacitorPairingPorts } from './native/capacitorPorts'
import './app.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App native={Capacitor.isNativePlatform() ? createCapacitorPairingPorts() : null} />
  </StrictMode>,
)
