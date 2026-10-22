package com.tea4go.claudemobileremote;

import android.Manifest;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.speech.RecognitionListener;
import android.speech.RecognizerIntent;
import android.speech.SpeechRecognizer;
import androidx.core.app.NotificationCompat;
import androidx.core.app.NotificationManagerCompat;
import com.getcapacitor.JSObject;
import com.getcapacitor.PermissionState;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;
import com.getcapacitor.annotation.Permission;
import com.getcapacitor.annotation.PermissionCallback;
import java.io.File;
import java.util.ArrayList;
import java.util.Map;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import org.json.JSONObject;

@CapacitorPlugin(name = "SessionExperience", permissions = {
    @Permission(alias = "microphone", strings = { Manifest.permission.RECORD_AUDIO }),
    @Permission(alias = "notifications", strings = { Manifest.permission.POST_NOTIFICATIONS })
})
public class SessionExperiencePlugin extends Plugin {
    private static final String CHANNEL = "cmr.session.events";
    private static final String SESSION_EXTRA = "cmr.notification.sessionId";
    private static final Map<String, String> BODIES = new java.util.HashMap<>();
    static {
        BODIES.put("approval", "有工具操作等待审批"); BODIES.put("question", "有问题等待回答");
        BODIES.put("completed", "Claude 已完成本轮任务"); BODIES.put("failed", "Claude 执行失败，请打开会话查看");
        BODIES.put("stopped", "Claude 会话已结束");
    }
    private final Handler main = new Handler(Looper.getMainLooper());
    private final ExecutorService io = Executors.newSingleThreadExecutor();
    private SessionDisplayCache cache;
    private SpeechRecognizer recognizer;
    private PluginCall speechCall;
    private String transcript = "";
    private String viewedSession = "";
    private boolean stopped;
    private volatile boolean foreground = true;
    private final Runnable speechTimeout = () -> finishSpeech(false);

    @Override public void load() {
        cache = new SessionDisplayCache(new File(getContext().getNoBackupFilesDir(), "session-display-cache"));
        manager().createNotificationChannel(new NotificationChannel(CHANNEL, "会话提醒", NotificationManager.IMPORTANCE_DEFAULT));
        handleNotificationIntent(getActivity().getIntent());
    }
    private NotificationManager manager() { return (NotificationManager) getContext().getSystemService(Context.NOTIFICATION_SERVICE); }
    private SharedPreferences preferences() { return getContext().getSharedPreferences("cmr.experience", Context.MODE_PRIVATE); }
    private boolean notificationsAllowed() {
        NotificationChannel channel = manager().getNotificationChannel(CHANNEL);
        return NotificationManagerCompat.from(getContext()).areNotificationsEnabled()
            && (channel == null || channel.getImportance() != NotificationManager.IMPORTANCE_NONE)
            && (Build.VERSION.SDK_INT < 33 || getPermissionState("notifications") == PermissionState.GRANTED);
    }
    private JSObject notificationState() {
        JSObject state = new JSObject();
        state.put("enabled", preferences().getBoolean("notifications.enabled", false) && notificationsAllowed());
        state.put("remoteConfigured", false); state.put("remoteReady", false);
        return state;
    }
    @PluginMethod public void notificationStatus(PluginCall call) { call.resolve(notificationState()); }
    @PluginMethod public void setNotifications(PluginCall call) { main.post(() -> updateNotifications(call)); }
    private void updateNotifications(PluginCall call) {
        if (!Boolean.TRUE.equals(call.getBoolean("enabled"))) {
            preferences().edit().putBoolean("notifications.enabled", false).apply();
            manager().cancelAll(); call.resolve(notificationState()); return;
        }
        if (Build.VERSION.SDK_INT >= 33 && getPermissionState("notifications") != PermissionState.GRANTED) {
            requestPermissionForAlias("notifications", call, "notificationPermission");
        } else enableNotifications(call);
    }
    @PermissionCallback private void notificationPermission(PluginCall call) { enableNotifications(call); }
    private void enableNotifications(PluginCall call) {
        preferences().edit().putBoolean("notifications.enabled", notificationsAllowed()).apply();
        call.resolve(notificationState());
    }
    @PluginMethod public void syncPushSessions(PluginCall call) {
        viewedSession = call.getString("viewedSession", "");
        call.resolve();
    }
    @PluginMethod public void notify(PluginCall call) {
        String sessionId = call.getString("sessionId", "");
        String kind = call.getString("kind", "");
        boolean validSequence = SessionNotificationPolicy.validSequence(call.getData().opt("seq"));
        if (!preferences().getBoolean("notifications.enabled", false) || !notificationsAllowed() ||
            sessionId.isEmpty() || sessionId.length() > 256 || !validSequence || !BODIES.containsKey(kind) || (foreground && sessionId.equals(viewedSession))) {
            call.resolve(); return;
        }
        try {
            Intent intent = new Intent(getContext(), MainActivity.class)
                .setAction("com.tea4go.claudemobileremote.OPEN_SESSION")
                .setData(Uri.parse("cmr-notification://session/" + Uri.encode(sessionId)))
                .putExtra(SESSION_EXTRA, sessionId)
                .addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP | Intent.FLAG_ACTIVITY_SINGLE_TOP);
            PendingIntent pending = PendingIntent.getActivity(getContext(), 0, intent, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
            android.app.Notification notification = new NotificationCompat.Builder(getContext(), CHANNEL)
                .setSmallIcon(R.drawable.ic_stat_session).setContentTitle("Claude Mobile Remote").setContentText(BODIES.get(kind))
                .setContentIntent(pending).setAutoCancel(true).setOnlyAlertOnce(false)
                .setVisibility(NotificationCompat.VISIBILITY_PRIVATE).build();
            manager().notify("cmr.session." + sessionId, 1, notification);
            call.resolve();
        } catch (Exception error) { call.reject("notification_unavailable"); }
    }
    @PluginMethod public void clearNotifications(PluginCall call) {
        String sessionId = call.getString("sessionId", "");
        manager().cancel("cmr.session." + sessionId, 1); call.resolve();
    }
    private void handleNotificationIntent(Intent intent) {
        if (intent == null || !"com.tea4go.claudemobileremote.OPEN_SESSION".equals(intent.getAction())) return;
        String sessionId = intent.getStringExtra(SESSION_EXTRA);
        intent.removeExtra(SESSION_EXTRA);
        if (sessionId == null || sessionId.isEmpty() || sessionId.length() > 256) return;
        JSObject data = new JSObject(); data.put("sessionId", sessionId);
        notifyListeners("notificationOpened", data, true);
    }
    @Override protected void handleOnNewIntent(Intent intent) { handleNotificationIntent(intent); }

    @PluginMethod public void recognize(PluginCall call) {
        main.post(() -> {
            if (speechCall != null) { call.reject("speech_busy"); return; }
            if (stopped) { call.reject("speech_unavailable"); return; }
            speechCall = call; transcript = "";
            if (getPermissionState("microphone") != PermissionState.GRANTED) requestPermissionForAlias("microphone", call, "microphonePermission");
            else startSpeech(call);
        });
    }
    @PermissionCallback private void microphonePermission(PluginCall call) {
        main.post(() -> {
            if (speechCall != call) return;
            if (getPermissionState("microphone") != PermissionState.GRANTED || stopped) failSpeech();
            else startSpeech(call);
        });
    }
    private void startSpeech(PluginCall call) {
        if (speechCall != call) return;
        try {
            if (!SpeechRecognizer.isRecognitionAvailable(getContext())) { failSpeech(); return; }
            recognizer = Build.VERSION.SDK_INT >= 31 && SpeechRecognizer.isOnDeviceRecognitionAvailable(getContext())
                ? SpeechRecognizer.createOnDeviceSpeechRecognizer(getContext()) : SpeechRecognizer.createSpeechRecognizer(getContext());
            recognizer.setRecognitionListener(new RecognitionListener() {
                private boolean current() { return speechCall == call; }
                private void accept(Bundle results) {
                    if (!current() || results == null) return;
                    ArrayList<String> values = results.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION);
                    if (values != null && !values.isEmpty()) transcript = values.get(0);
                }
                public void onResults(Bundle results) { if (current()) { accept(results); finishSpeech(false); } }
                public void onPartialResults(Bundle results) { accept(results); }
                public void onError(int error) { if (current()) { if (!transcript.isEmpty()) finishSpeech(false); else failSpeech(); } }
                public void onReadyForSpeech(Bundle params) {}
                public void onBeginningOfSpeech() {}
                public void onRmsChanged(float rmsdB) {}
                public void onBufferReceived(byte[] buffer) {}
                public void onEndOfSpeech() {}
                public void onEvent(int eventType, Bundle params) {}
            });
            Intent intent = new Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH)
                .putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
                .putExtra(RecognizerIntent.EXTRA_LANGUAGE, "zh-CN").putExtra(RecognizerIntent.EXTRA_PARTIAL_RESULTS, true);
            recognizer.startListening(intent);
            main.postDelayed(speechTimeout, 60000);
        } catch (Exception error) { failSpeech(); }
    }
    private PluginCall cleanupSpeech() {
        PluginCall call = speechCall; speechCall = null;
        main.removeCallbacks(speechTimeout);
        if (recognizer != null) { recognizer.cancel(); recognizer.destroy(); recognizer = null; }
        return call;
    }
    private void finishSpeech(boolean cancelled) {
        String text = cancelled ? "" : transcript;
        PluginCall call = cleanupSpeech(); transcript = "";
        if (call != null) { JSObject result = new JSObject(); result.put("text", text); call.resolve(result); }
    }
    private void failSpeech() { PluginCall call = cleanupSpeech(); transcript = ""; if (call != null) call.reject("speech_unavailable"); }
    @PluginMethod public void stopSpeech(PluginCall call) { main.post(() -> { finishSpeech(false); call.resolve(); }); }
    @PluginMethod public void cancelSpeech(PluginCall call) { main.post(() -> { finishSpeech(true); call.resolve(); }); }
    @Override protected void handleOnResume() { stopped = false; foreground = true; }
    @Override protected void handleOnPause() { foreground = false; if (recognizer != null) finishSpeech(true); }
    @Override protected void handleOnStop() { stopped = true; finishSpeech(true); }
    @Override protected void handleOnDestroy() { stopped = true; finishSpeech(true); io.shutdown(); }

    private interface CacheOperation { JSObject run() throws Exception; }
    private void cacheTask(PluginCall call, CacheOperation operation) {
        io.execute(() -> {
            try { call.resolve(operation.run()); }
            catch (Exception error) { call.reject("本地消息缓存暂不可用"); }
        });
    }
    @PluginMethod public void readSessionCache(PluginCall call) {
        cacheTask(call, () -> { JSObject data = new JSObject(); String value = cache.read(call.getString("sessionId"), System.currentTimeMillis()); data.put("data", value == null ? JSONObject.NULL : value); return data; });
    }
    @PluginMethod public void writeSessionCache(PluginCall call) {
        cacheTask(call, () -> {
            String sessionId = call.getString("sessionId"), data = call.getString("data");
            if (data == null || data.getBytes(java.nio.charset.StandardCharsets.UTF_8).length > SessionDisplayCache.FILE_LIMIT) throw new IllegalArgumentException();
            JSONObject envelope = new JSONObject(data);
            if (!sessionId.equals(envelope.optString("sessionId")) || envelope.optInt("version") != 1) throw new IllegalArgumentException();
            cache.write(sessionId, data, System.currentTimeMillis()); return new JSObject();
        });
    }
    @PluginMethod public void removeSessionCache(PluginCall call) { cacheTask(call, () -> { cache.remove(call.getString("sessionId")); return new JSObject(); }); }
    @PluginMethod public void clearSessionCache(PluginCall call) { cacheTask(call, () -> { cache.clear(); return new JSObject(); }); }
}
