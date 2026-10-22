import { nativeLifecycle } from '../native/lifecycle'
import { AppShell } from './AppShell'

@Entry
@Component
struct Index {
  onBackPress(): boolean { return nativeLifecycle.onBack?.() ?? false }
  build() {
    Column() {
      AppShell()
    }
    .width('100%')
    .height('100%')
  }
}
