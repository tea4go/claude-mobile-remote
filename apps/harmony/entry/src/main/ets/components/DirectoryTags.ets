import type { ProjectGroup } from '../session/projectGroups'
import { SECONDARY } from '../ui/styles'

interface VisibleTag { key: string; title: string; width: number }

@Component
export struct DirectoryTags {
  @Prop @Watch('layoutTags') projects: Array<ProjectGroup> = []
  @State private visible: Array<VisibleTag> = []
  @State private overflow: boolean = false
  private available: number = 0

  private textWidth(title: string): number {
    return Math.ceil(this.getUIContext().px2vp(this.getUIContext().getMeasureUtils().measureText({ textContent: title, fontSize: 11 })))
  }

  private layoutTags(): void {
    if (this.available <= 0) return
    const tags = this.projects.map((project: ProjectGroup): VisibleTag =>
      ({ key: project.key, title: project.title, width: this.textWidth(project.title) + 12 }))
    const gap = 4
    const total = tags.reduce((width: number, tag: VisibleTag) => width + tag.width + gap, -gap)
    this.overflow = total > this.available
    const visible: Array<VisibleTag> = []
    let remaining = this.available - (this.overflow ? this.textWidth('...') + gap : 0)
    for (const tag of tags) {
      if (remaining <= 12) break
      visible.push({ key: tag.key, title: tag.title, width: Math.min(tag.width, remaining) })
      remaining -= tag.width + gap
    }
    this.visible = visible
  }

  build() {
    Row({ space: 4 }) {
      ForEach(this.visible, (tag: VisibleTag) => {
        Text(tag.title).fontSize(11).fontColor(SECONDARY).maxLines(1)
          .textOverflow({ overflow: TextOverflow.Clip }).width(tag.width).padding({ left: 5, right: 5, top: 3, bottom: 3 })
          .borderRadius(8).backgroundColor($r('app.color.surface_subtle')).border({ width: 1, color: $r('app.color.border') })
      }, (tag: VisibleTag) => JSON.stringify([tag.key, tag.title, tag.width]))
      if (this.overflow) { Text('...').fontSize(11).fontColor(SECONDARY).maxLines(1) }
    }.width('100%').height(23).clip(true).onAreaChange((_previous: Area, current: Area) => {
      const width = Number(current.width)
      if (this.available !== width) { this.available = width; this.layoutTags() }
    })
  }
}
