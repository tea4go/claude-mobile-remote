package com.tea4go.claudemobileremote;

import android.os.Bundle;
import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {
    @Override public void onCreate(Bundle savedInstanceState) {
        registerPlugin(SessionExperiencePlugin.class);
        super.onCreate(savedInstanceState);
    }
}
