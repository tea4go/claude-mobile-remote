package com.tea4go.claudemobileremote;

import android.content.Context;
import com.getcapacitor.JSObject;
import org.json.JSONArray;

// No SDK initialization, route persistence, or notification network traffic.
final class SessionPushManager {
    private static final SessionPushManager INSTANCE = new SessionPushManager();
    static SessionPushManager get(Context context) { return INSTANCE; }
    JSObject status() {
        JSObject state = new JSObject();
        state.put("remoteConfigured", false);
        state.put("remoteEnabled", false);
        state.put("remoteReady", false);
        return state;
    }
    void setEnabled(boolean enabled) {}
    void refresh() {}
    void sessions(JSONArray sessions) {}
    String session(String topic) { return null; }
    void resumed() {}
    void paused() {}
    void dismiss(String session) {}
}
