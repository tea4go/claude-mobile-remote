package com.tea4go.claudemobileremote;

final class SessionNotificationPolicy {
    private SessionNotificationPolicy() {}
    static String topic(String session, String token) throws Exception {
        byte[] hash = java.security.MessageDigest.getInstance("SHA-256").digest(("cmr-push-v1\0" + session + "\0" + token).getBytes(java.nio.charset.StandardCharsets.UTF_8));
        StringBuilder value = new StringBuilder("cmr_");
        for (int i = 0; i < 16; i++) value.append(String.format(java.util.Locale.ROOT, "%02x", hash[i] & 255));
        return value.toString();
    }
    static boolean canOpen(String topic, long expires, boolean enabled, long now) {
        return enabled && expires > now && topic != null && topic.matches("cmr_[0-9a-f]{32}");
    }
    // Android's JSON parser uses Integer for small JSON numbers and Long for larger ones.
    // Capacitor PluginCall.getLong alone silently defaults Integer sequences to zero.
    static boolean validSequence(Object value) {
        if (!(value instanceof Number)) return false;
        double seq = ((Number) value).doubleValue();
        return Double.isFinite(seq) && seq > 0 && seq <= 9007199254740991D && Math.rint(seq) == seq;
    }
}
