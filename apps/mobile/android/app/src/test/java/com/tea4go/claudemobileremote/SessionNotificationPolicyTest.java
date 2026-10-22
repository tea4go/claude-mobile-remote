package com.tea4go.claudemobileremote;

import org.junit.Test;
import static org.junit.Assert.*;

public class SessionNotificationPolicyTest {
    @Test public void acceptsSmallIntegerAndLongJsonSequences() {
        assertTrue(SessionNotificationPolicy.validSequence(Integer.valueOf(1)));
        assertTrue(SessionNotificationPolicy.validSequence(Long.valueOf(2147483648L)));
        assertTrue(SessionNotificationPolicy.validSequence(Double.valueOf(2)));
    }
    @Test public void rejectsMissingFractionalNonFiniteAndUnsafeSequences() {
        for (Object seq : new Object[]{null,"1",0,-1,1.5,Double.NaN,Double.POSITIVE_INFINITY,9007199254740992D}) assertFalse(SessionNotificationPolicy.validSequence(seq));
    }
}
