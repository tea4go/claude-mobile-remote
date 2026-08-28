import type { CSSProperties } from 'react'

// 共享视觉常量：按钮尺寸集中一处，避免各组件抄出不一致的内边距与字号。
// antd-mobile 的默认档按中文字号偏大（17px 字 + 7px 上下内边距），
// 这里用行内样式覆盖，保持点按区域可用但不再抢视觉。

// 行内次要操作：取消、重命名、审批决议等跟随文字排布的按钮。
export const COMPACT_BUTTON_STYLE: CSSProperties = {
  fontSize: 13,
  lineHeight: 1.4,
  padding: '4px 12px',
  borderRadius: 6,
}

// 独占一行的主操作：配对页连接、列表新增配对。比行内按钮高一档，
// 仍明显小于 antd-mobile 默认档。
export const BLOCK_BUTTON_STYLE: CSSProperties = {
  fontSize: 14,
  lineHeight: 1.4,
  padding: '8px 12px',
  borderRadius: 8,
}

// 页面底色：偏白但不纯白。留一点灰度是刻意的——消息流里的白底元素
// （Markdown 表格、玻璃面板）靠它才能浮起来；纯白底会让表格重新与
// 背景糊成一片。
export const PAGE_BACKGROUND = '#fafbfd'

// 会话详情页底色：玻璃表面需要背后有层次才看得出模糊，因此用自上而下
// 收白的清浅渐变，最深处也只比 PAGE_BACKGROUND 低一档。
export const SESSION_BACKGROUND =
  'linear-gradient(180deg, #f2f5fa 0%, #f8fafc 45%, #fdfefe 100%)'

// 半透明玻璃表面：导航栏与固定输入栏共用。backdropFilter 是纯 CSS
// 能力，不依赖 antd-mobile；不支持的老 WebView 会退化为半透明纯色，
// 因此背景色本身已保证文字对比度。
export const GLASS_SURFACE_STYLE: CSSProperties = {
  background: 'rgba(255, 255, 255, 0.72)',
  backdropFilter: 'blur(20px) saturate(180%)',
  WebkitBackdropFilter: 'blur(20px) saturate(180%)',
} as CSSProperties

// 抽屉内的分区容器：坐在透亮玻璃上，用白色高光边而不是深色描边分层，
// 深色描边在亮玻璃上会显脏。
export const GLASS_PANEL_STYLE: CSSProperties = {
  background: 'rgba(255, 255, 255, 0.7)',
  border: '1px solid rgba(255, 255, 255, 0.8)',
  borderRadius: 10,
  padding: '10px 12px',
}

// 抽屉遮罩：必须比 antd-mobile 默认档淡得多。抽屉自身开了
// backdropFilter，而遮罩就在它背后——默认的 rgba(0,0,0,0.55) 会被采样
// 进模糊结果，把整个抽屉染成灰面（这正是「玻璃做不出来」的原因）。
// 遮罩仍然存在，因此点击拦截与 onMaskClick 行为不变。
export const DRAWER_MASK_STYLE: CSSProperties = {
  background: 'rgba(15, 23, 42, 0.16)',
}

// 底部抽屉的容器：活动详情与待处理事项共用，限高后由内部自行滚动。
// 比 GLASS_SURFACE_STYLE 更透亮：白底压到 0.6 让背后内容透出来，靠
// brightness 把采样到的暗部提亮，再用顶部高光边和外投影把玻璃“抬”起来。
export const DRAWER_SHEET_STYLE: CSSProperties = {
  background: 'rgba(255, 255, 255, 0.6)',
  backdropFilter: 'blur(30px) saturate(200%) brightness(1.14)',
  WebkitBackdropFilter: 'blur(30px) saturate(200%) brightness(1.14)',
  borderTop: '1px solid rgba(255, 255, 255, 0.75)',
  boxShadow: '0 -12px 40px rgba(15, 23, 42, 0.12)',
  borderTopLeftRadius: 16,
  borderTopRightRadius: 16,
  maxHeight: '72vh',
  display: 'flex',
  flexDirection: 'column',
} as CSSProperties

// 抽屉标题行：左右各留一个图标位，标题居中。分隔线取极淡的冷灰，
// 在透亮玻璃上不留脏边。
export const DRAWER_HEADER_STYLE: CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 8,
  padding: '12px 12px 8px',
  borderBottom: '1px solid rgba(15, 23, 42, 0.06)',
}

// 抽屉标题行上的返回/关闭按钮：点按区域 32px，视觉只有一个符号。
export const ICON_BUTTON_STYLE: CSSProperties = {
  width: 32,
  height: 32,
  flexShrink: 0,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  border: 'none',
  background: 'none',
  color: '#333',
  fontSize: 17,
  fontFamily: 'inherit',
  lineHeight: 1,
  cursor: 'pointer',
}
