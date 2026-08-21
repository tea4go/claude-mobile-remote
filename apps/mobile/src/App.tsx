import { NavBar, Tag } from 'antd-mobile'

function App() {
  return (
    <div style={{ minHeight: '100vh', background: '#f5f5f5' }}>
      <NavBar back={null}>Claude Mobile Remote</NavBar>
      <div style={{ padding: 16 }}>
        <Tag color="default">A1 骨架 · 会话接力将在 A5 实现</Tag>
      </div>
    </div>
  )
}

export default App
