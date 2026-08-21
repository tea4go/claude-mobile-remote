import { render, screen } from '@testing-library/react'
import App from './App'

test('renders skeleton title', () => {
  render(<App />)
  expect(screen.getByText('Claude Mobile Remote')).toBeInTheDocument()
})
