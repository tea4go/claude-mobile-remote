package com.tea4go.claudemobileremote;

import android.content.Context;
import android.content.SharedPreferences;
import android.os.Handler;
import android.os.Looper;
import androidx.core.app.NotificationManagerCompat;
import cn.jpush.android.api.JPushInterface;
import com.getcapacitor.JSObject;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.util.Iterator;
import java.util.Locale;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import javax.net.ssl.HttpsURLConnection;
import org.json.JSONArray;
import org.json.JSONObject;

// All mutation and reconciliation are serialized. Only notification routes are persisted;
// pairing tokens are hashed in memory and remain in the existing secure pairing store.
final class SessionPushManager {
    private static SessionPushManager instance;
    static synchronized SessionPushManager get(Context context) {
        if (instance == null) instance = new SessionPushManager(context.getApplicationContext());
        return instance;
    }
    private final Context context;
    private final SharedPreferences prefs;
    private final Handler main = new Handler(Looper.getMainLooper());
    private final ExecutorService io = Executors.newSingleThreadExecutor();
    private JSONObject routes;
    private JSONObject applied;
    private boolean started, syncing;
    private int retries;
    private volatile boolean ready;
    private volatile String error = "";
    private final Runnable expiry = () -> { synchronized (this) { prune(); sync(); } };
    private SessionPushManager(Context context) {
        this.context = context;
        prefs = context.getSharedPreferences("cmr.experience", Context.MODE_PRIVATE);
        routes = read("push.routes"); applied = read("push.applied");
        prune();
    }
    private JSONObject read(String key) {
        try { return new JSONObject(prefs.getString(key, "{}")); } catch (Exception ignored) { return new JSONObject(); }
    }
    synchronized boolean enabled() { return prefs.getBoolean("notifications.enabled", false) && prefs.getBoolean("push.consent.v1", false); }
    private boolean allowed() { return NotificationManagerCompat.from(context).areNotificationsEnabled(); }
    synchronized JSObject status() {
        JSObject result = new JSObject();
        result.put("vendorConfigured", vendorConfigured());
        result.put("remoteConfigured", true); result.put("remoteEnabled", enabled());
        result.put("remoteReady", ready && enabled() && allowed()); result.put("pushError", error);
        return result;
    }
    private boolean vendorConfigured() {
        String maker = android.os.Build.MANUFACTURER.toLowerCase(Locale.ROOT);
        if (maker.contains("xiaomi") || maker.contains("redmi")) return BuildConfig.CMR_XIAOMI_PUSH_CONFIGURED;
        if (maker.contains("huawei")) return BuildConfig.CMR_HUAWEI_PUSH_CONFIGURED;
        if (maker.contains("honor")) return BuildConfig.CMR_HONOR_PUSH_CONFIGURED;
        if (maker.contains("oppo") || maker.contains("oneplus") || maker.contains("realme")) return BuildConfig.CMR_OPPO_PUSH_CONFIGURED;
        if (maker.contains("vivo") || maker.contains("iqoo")) return BuildConfig.CMR_VIVO_PUSH_CONFIGURED;
        return false;
    }
    synchronized void setEnabled(boolean value) {
        prefs.edit().putBoolean("notifications.enabled", value).putBoolean("push.consent.v1", value).apply();
        retries = 0;
        if (!value && started) JPushInterface.stopPush(context);
        refresh();
    }
    synchronized void refresh() {
        prune(); retries = 0;
        if (enabled() && allowed()) {
            if (!started) {
                JPushInterface.setDebugMode(false);
                cn.jpush.android.api.BasicPushNotificationBuilder builder = new cn.jpush.android.api.BasicPushNotificationBuilder(context);
                builder.statusBarDrawable = R.drawable.ic_stat_session;
                JPushInterface.setDefaultPushNotificationBuilder(builder);
                JPushInterface.init(context);
                started = true;
            }
            JPushInterface.resumePush(context);
        } else if (started) JPushInterface.stopPush(context);
        sync();
    }
    private static long date(String value) throws Exception {
        return java.time.Instant.parse(value).toEpochMilli();
    }
    synchronized void sessions(JSONArray sessions) throws Exception {
        if (sessions == null || sessions.length() > 1000) throw new IllegalArgumentException();
        JSONObject next = new JSONObject();
        for (int i = 0; i < sessions.length(); i++) {
            JSONObject item = sessions.getJSONObject(i);
            String id = item.getString("sessionId"), token = item.getString("pairToken"), expires = item.getString("expiresAt");
            if (id.isEmpty() || id.length() > 256 || token.isEmpty() || token.length() > 1024) throw new IllegalArgumentException();
            if (date(expires) > System.currentTimeMillis()) next.put(SessionNotificationPolicy.topic(id, token), new JSONObject().put("session", id).put("expires", expires));
        }
        routes = next; retries = 0; save(); prune(); sync();
    }
    synchronized String session(String topic) {
        if (!enabled() || topic == null || !topic.matches("cmr_[0-9a-f]{32}")) return null;
        try { JSONObject route = routes.getJSONObject(topic); return SessionNotificationPolicy.canOpen(topic, date(route.getString("expires")), enabled(), System.currentTimeMillis()) ? route.getString("session") : null; }
        catch (Exception ignored) { return null; }
    }
    synchronized void resumed() { if (started && enabled()) JPushInterface.onResume(context); }
    synchronized void paused() { if (started && enabled()) JPushInterface.onPause(context); }
    synchronized void arrived(String topic, int id) {
        if (session(topic) != null) prefs.edit().putInt("push.notice." + topic, id).apply();
    }
    synchronized void dismiss(String session) {
        Iterator<String> keys = routes.keys();
        while (keys.hasNext()) {
            String topic = keys.next();
            if (session.equals(routes.optJSONObject(topic).optString("session"))) {
                int id = prefs.getInt("push.notice." + topic, 0);
                if (id != 0 && started) JPushInterface.clearNotificationById(context, id);
                prefs.edit().remove("push.notice." + topic).apply();
            }
        }
    }
    private void save() { prefs.edit().putString("push.routes", routes.toString()).putString("push.applied", applied.toString()).apply(); }
    private void prune() {
        long next = Long.MAX_VALUE;
        Iterator<String> keys = routes.keys();
        while (keys.hasNext()) {
            try { long time = date(routes.getJSONObject(keys.next()).getString("expires")); if (time <= System.currentTimeMillis()) keys.remove(); else next = Math.min(next, time); }
            catch (Exception ignored) { keys.remove(); }
        }
        save(); main.removeCallbacks(expiry);
        if (next != Long.MAX_VALUE) main.postDelayed(expiry, Math.max(1000, Math.min(86400000L, next - System.currentTimeMillis())));
    }
    private void sync() {
        ready = false;
        if (syncing) return;
        String rid = started && enabled() && allowed() ? JPushInterface.getRegistrationID(context) : "";
        String key = null, method = null; JSONObject entry = null;
        Iterator<String> old = applied.keys();
        while (old.hasNext()) {
            String candidate = old.next(); JSONObject value = applied.optJSONObject(candidate);
            if (value == null || value.optJSONObject("route") == null) { old.remove(); continue; }
            String topic = value.optString("topic");
            if (!rid.equals(value.optString("rid")) || !routes.has(topic) || !routes.optJSONObject(topic).optString("expires").equals(value.optJSONObject("route").optString("expires"))) {
                key = candidate; entry = value; method = "DELETE"; break;
            }
        }
        if (key == null && !rid.isEmpty()) {
            Iterator<String> desired = routes.keys();
            while (desired.hasNext()) {
                String topic = desired.next(), candidate = rid + ":" + topic;
                if (!applied.has(candidate) || !applied.optJSONObject(candidate).optBoolean("confirmed", false)) {
                    try { entry = new JSONObject().put("topic", topic).put("rid", rid).put("route", routes.getJSONObject(topic)); }
                    catch (Exception ignored) { return; }
                    key = candidate; method = "PUT"; break;
                }
            }
        }
        if (key == null) { ready = !rid.isEmpty(); error = ""; return; }
        final String selected = key, operation = method;
        final JSONObject value = entry;
        // Persist an attempted PUT too: a timeout can hide server success. Later
        // removal/off must DELETE that possible registration rather than forget it.
        if (operation.equals("PUT")) {
            try { value.put("confirmed", false); applied.put(selected, value); save(); }
            catch (Exception ignored) { return; }
        }
        syncing = true;
        io.execute(() -> {
            boolean success = send(operation, value);
            synchronized (SessionPushManager.this) {
                syncing = false;
                if (success) {
                    if (operation.equals("DELETE")) applied.remove(selected);
                    else try { value.put("confirmed", true); applied.put(selected, value); } catch (Exception ignored) {}
                    save(); retries = 0; error = ""; sync();
                } else {
                    error = "push_subscription_failed";
                    if (retries++ < 3) main.postDelayed(() -> { synchronized (SessionPushManager.this) { sync(); } }, 2000);
                }
            }
        });
    }
    private boolean send(String method, JSONObject entry) {
        HttpsURLConnection connection = null;
        try {
            JSONObject body = new JSONObject().put("topic", entry.getString("topic")).put("registrationId", entry.getString("rid"));
            if (method.equals("PUT")) body.put("expiresAt", entry.getJSONObject("route").getString("expires"));
            connection = (HttpsURLConnection) new URL("https://edo.iwhalecloud.com/verfiles/notify/v1/events").openConnection();
            connection.setInstanceFollowRedirects(false); connection.setConnectTimeout(10000); connection.setReadTimeout(10000);
            connection.setRequestMethod(method); connection.setRequestProperty("Content-Type", "application/json"); connection.setDoOutput(true);
            byte[] data = body.toString().getBytes(StandardCharsets.UTF_8); connection.setFixedLengthStreamingMode(data.length);
            try (java.io.OutputStream output = connection.getOutputStream()) { output.write(data); }
            return connection.getResponseCode() == 204;
        } catch (Exception ignored) { return false; }
        finally { if (connection != null) connection.disconnect(); }
    }
}
