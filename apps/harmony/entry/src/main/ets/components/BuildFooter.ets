import { APP_BUILD_LABEL } from '../common/BuildInfo'
import { SECONDARY } from '../ui/styles'

@Component
export struct BuildFooter {
  build() {
    Text(APP_BUILD_LABEL).width('100%').textAlign(TextAlign.Center).fontSize(11)
      .fontColor(SECONDARY).padding({ left: 16, right: 16, top: 4, bottom: 8 })
  }
}
