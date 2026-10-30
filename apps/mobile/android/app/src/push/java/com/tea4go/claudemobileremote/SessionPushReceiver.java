package com.tea4go.claudemobileremote;

import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import cn.jpush.android.api.NotificationMessage;
import cn.jpush.android.service.JPushMessageReceiver;
import org.json.JSONObject;

public final class SessionPushReceiver extends JPushMessageReceiver {
    @Override public boolean isNeedShowNotification(Context context, NotificationMessage message, String source) {
        if (RemoteApplication.isForeground()) return false;
        try { return SessionPushManager.get(context).session(new JSONObject(message.notificationExtras).optString("topic")) != null; }
        catch (Exception ignored) { return false; }
    }
    @Override public void onRegister(Context context, String id) { SessionPushManager.get(context).refresh(); }
    @Override public void onConnected(Context context, boolean connected) { if (connected) SessionPushManager.get(context).refresh(); }
    @Override public void onNotifyMessageArrived(Context context, NotificationMessage message) {
        try { SessionPushManager.get(context).arrived(new JSONObject(message.notificationExtras).optString("topic"), message.notificationId); }
        catch (Exception ignored) {}
    }
    @Override public void onNotifyMessageOpened(Context context, NotificationMessage message) {
        try {
            String topic = new JSONObject(message.notificationExtras).optString("topic");
            if (SessionPushManager.get(context).session(topic) == null) return;
            context.startActivity(new Intent(context, MainActivity.class).setAction(Intent.ACTION_VIEW)
                .setData(Uri.parse("cmr-push://notification/" + topic))
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_CLEAR_TOP | Intent.FLAG_ACTIVITY_SINGLE_TOP));
        } catch (Exception ignored) { /* Never log provider payloads. */ }
    }
}
