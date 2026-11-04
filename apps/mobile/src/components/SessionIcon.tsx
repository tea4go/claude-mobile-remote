import camera from '../assets/icons/svgs/session/camera.svg'
import image from '../assets/icons/svgs/session/image.svg'
import file from '../assets/icons/svgs/session/file.svg'
import type { CSSProperties } from 'react'
import arrow from '../assets/icons/svgs/session/arrow-right.svg'
import logo from '../assets/icons/svgs/session/claude_sse_logo.svg'
import clear from '../assets/icons/svgs/session/clear_cache.svg'
import debug from '../assets/icons/svgs/session/debug.svg'
import demo from '../assets/icons/svgs/session/demo_introduce.svg'
import folder from '../assets/icons/svgs/session/folder_expand.svg'
import folderClosed from '../assets/icons/svgs/session/folder_collapse.svg'
import add from '../assets/icons/svgs/session/folder_create_new_session.svg'
import back from '../assets/icons/svgs/session/go_back.svg'
import steps from '../assets/icons/svgs/session/list_group.svg'
import microphone from '../assets/icons/svgs/session/microphone.svg'
import notify from '../assets/icons/svgs/session/notify.svg'
import paperclip from '../assets/icons/svgs/session/paperclip.svg'
import pc from '../assets/icons/svgs/session/pc_host.svg'
import pcSettings from '../assets/icons/svgs/session/pc_setting.svg'
import permission from '../assets/icons/svgs/session/permission.svg'
import scan from '../assets/icons/svgs/session/scan_qcode.svg'
import send from '../assets/icons/svgs/session/send.svg'
import settings from '../assets/icons/svgs/session/setting.svg'
import target from '../assets/icons/svgs/session/target_pc_ip.svg'

const icons = { camera, image, file, arrow, logo, clear, debug, demo, folder, folderClosed, add, back, steps, microphone, notify, paperclip, pc, pcSettings, permission, scan, send, settings, target }

export default function SessionIcon({ name, size = 20, style }: { name: keyof typeof icons; size?: number; style?: CSSProperties }) {
  const mask = `url("${icons[name]}") center / contain no-repeat`
  return <span aria-hidden="true" style={{ display: 'inline-block', flexShrink: 0, width: size, height: size, backgroundColor: 'currentColor', mask, WebkitMask: mask, verticalAlign: 'middle', ...style }} />
}
