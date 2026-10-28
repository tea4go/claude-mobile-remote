# Harmony 图标资源

依据华为通用 UX 标准 2.1.4.3.1：前景和背景均为1024×1024，背景不得透明，资源不预裁圆角、不额外添加内间距。

- `app-icon.svg`：真正的矢量合成预览源，无嵌入位图和辅助网格。
- `app-icon-216.png`：216×216的合成展示图，供后台替换；不替代包内分层图标。
- AppScope及entry中的`foreground.png`：透明底蓝色主体。
- 两处`background.png`：不透明、铺满画布的白色背景。
- entry中的`startIcon.png`：1024×1024合成图，保持启动图一致。

从仓库根目录重新生成（macOS）：

```sh
swift -module-cache-path /tmp/cmr-icon-module-cache apps/harmony/branding/generate-icons.swift
```

生成器的共用多边形数据同时输出SVG和PNG，避免两种格式形状不一致。系统负责图标最终形状裁切。主体周围空白属于构图，背景始终铺满1024画布，未增加额外透明边框。

2026-10-28已重新构建1.0.3签名APP并核验包内新图标；1.0.2仍为旧图标。尚未上传或完成平台自检。
