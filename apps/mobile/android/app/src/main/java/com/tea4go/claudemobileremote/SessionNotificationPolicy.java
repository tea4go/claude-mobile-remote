package com.tea4go.claudemobileremote;

final class SessionNotificationPolicy {
    private SessionNotificationPolicy() {}
    // Android's JSON parser uses Integer for small JSON numbers and Long for larger ones.
    // Capacitor PluginCall.getLong alone silently defaults Integer sequences to zero.
    static boolean validSequence(Object value) {
        if (!(value instanceof Number)) return false;
        double seq = ((Number) value).doubleValue();
        return Double.isFinite(seq) && seq > 0 && seq <= 9007199254740991D && Math.rint(seq) == seq;
    }
}
