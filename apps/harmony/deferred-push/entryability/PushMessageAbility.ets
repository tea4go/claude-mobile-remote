import { UIAbility } from '@kit.AbilityKit'
import { pushService, pushCommon } from '@kit.PushKit'
import { JPushInterface } from '@jg/push'

// Required by Push Kit for the server's display_foreground=0 policy.
export default class PushMessageAbility extends UIAbility {
  onCreate(): void {
    try {
      pushService.receiveMessage('DEFAULT', this, async (data: pushCommon.PushPayload) => {
        await JPushInterface.defaultMessageBackgroundData(data)
      })
    } catch (_) { /* No payload logging. */ }
  }
}
