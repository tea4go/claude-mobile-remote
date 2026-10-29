package com.tea4go.claudemobileremote;

import org.junit.Test;
import static org.junit.Assert.*;

public class SessionPushPolicyTest {
    @Test public void routingMatchesSharedPCContract() throws Exception {
        String topic = SessionNotificationPolicy.topic("fixture-session", "fixture-pairing");
        assertEquals("cmr_c0ffbb23aa5fec6594bfcf4b6b452eb0", topic);
        assertNotEquals(topic, SessionNotificationPolicy.topic("other-session", "fixture-pairing"));
        assertNotEquals(topic, SessionNotificationPolicy.topic("fixture-session", "rotated-pairing"));
        assertFalse(topic.contains("fixture"));
    }
    @Test public void clickRequiresConsentAndUnexpiredOpaqueRoute() {
        String topic = "cmr_00000000000000000000000000000000";
        assertTrue(SessionNotificationPolicy.canOpen(topic, 101, true, 100));
        assertFalse(SessionNotificationPolicy.canOpen(topic, 100, true, 100));
        assertFalse(SessionNotificationPolicy.canOpen(topic, 99, true, 100));
        assertFalse(SessionNotificationPolicy.canOpen(topic, 101, false, 100));
        assertFalse(SessionNotificationPolicy.canOpen("raw-session", 101, true, 100));
        assertFalse(SessionNotificationPolicy.canOpen(null, 101, true, 100));
    }
}
