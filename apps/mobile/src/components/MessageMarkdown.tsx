import Markdown, { type Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'

interface MessageMarkdownProps {
  text: string
}

// 安全约束（B1）：不启用 rehype-raw，raw HTML 一律按纯文本转义；
// 不使用 dangerouslySetInnerHTML；禁用 img 以避免加载远程资源；
// URL 由 react-markdown 内建过滤，javascript: 等协议不会产出可执行 href。
const DISALLOWED_ELEMENTS = ['img']

const codeBlockStyle: React.CSSProperties = {
  margin: '8px 0',
  padding: '8px 10px',
  background: '#f6f8fa',
  borderRadius: 6,
  overflowX: 'auto',
  whiteSpace: 'pre',
  fontSize: 12,
  lineHeight: 1.5,
}

const inlineCodeStyle: React.CSSProperties = {
  padding: '1px 4px',
  background: '#f0f0f0',
  borderRadius: 4,
  fontSize: 12,
  wordBreak: 'break-all',
}

const components: Components = {
  // 外链统一新窗口打开并阻断 referrer 与 window.opener。
  a: ({ children, href }) => (
    <a href={href} target="_blank" rel="noreferrer noopener">
      {children}
    </a>
  ),
  // 段落保留软换行，贴合 Claude 流式文本的原始排版。
  p: ({ children }) => <p style={{ margin: '4px 0', whiteSpace: 'pre-wrap' }}>{children}</p>,
  pre: ({ children }) => <pre style={codeBlockStyle}>{children}</pre>,
  // 代码块由 pre 提供容器样式，只给行内代码加背景。
  code: ({ children, className }) =>
    className === undefined ? (
      <code style={inlineCodeStyle}>{children}</code>
    ) : (
      <code className={className}>{children}</code>
    ),
  table: ({ children }) => (
    <div style={{ overflowX: 'auto', margin: '4px 0' }}>
      <table style={{ borderCollapse: 'collapse', fontSize: 12 }}>{children}</table>
    </div>
  ),
  th: ({ children }) => (
    <th style={{ border: '1px solid #ddd', padding: '4px 8px', background: '#fafafa', whiteSpace: 'nowrap' }}>
      {children}
    </th>
  ),
  td: ({ children }) => (
    <td style={{ border: '1px solid #ddd', padding: '4px 8px', whiteSpace: 'nowrap' }}>{children}</td>
  ),
  // 列表属于正文，保持随宽度自然折行；只有表格和代码块走横向滚动。
  ul: ({ children }) => <ul style={{ margin: '4px 0', paddingLeft: 20 }}>{children}</ul>,
  ol: ({ children }) => <ol style={{ margin: '4px 0', paddingLeft: 20 }}>{children}</ol>,
}

// Claude 文本的受限 Markdown 渲染（B1）：只支持 Markdown 文本、GFM
// 表格/删除线和代码块；HarmonyOS NEXT 复用同一受限语法语义，不复用本组件。
// 表格与代码块不擅自折行，溢出由容器提供横向滚动，便于阅读。
export default function MessageMarkdown({ text }: MessageMarkdownProps) {
  return (
    <div>
      <Markdown
        remarkPlugins={[remarkGfm]}
        disallowedElements={DISALLOWED_ELEMENTS}
        components={components}
      >
        {text}
      </Markdown>
    </div>
  )
}
