package com.tea4go.claudemobileremote;

import android.app.Activity;
import android.app.Application;
import android.os.Bundle;

public final class RemoteApplication extends Application {
    private static volatile boolean foreground;
    static boolean isForeground() { return foreground; }
    @Override public void onCreate() {
        super.onCreate();
        registerActivityLifecycleCallbacks(new ActivityLifecycleCallbacks() {
            private int visible;
            public void onActivityStarted(Activity activity) { foreground = ++visible > 0; }
            public void onActivityStopped(Activity activity) { foreground = Math.max(0, --visible) > 0; }
            public void onActivityCreated(Activity activity, Bundle state) {}
            public void onActivityResumed(Activity activity) {}
            public void onActivityPaused(Activity activity) {}
            public void onActivitySaveInstanceState(Activity activity, Bundle state) {}
            public void onActivityDestroyed(Activity activity) {}
        });
        SessionPushManager.get(this).refresh();
    }
}
