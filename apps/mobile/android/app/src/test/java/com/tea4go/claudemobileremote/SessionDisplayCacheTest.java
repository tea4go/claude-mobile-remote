package com.tea4go.claudemobileremote;

import org.junit.Test;
import org.junit.Rule;
import org.junit.rules.TemporaryFolder;
import java.io.File;
import java.nio.file.Files;
import java.nio.charset.StandardCharsets;
import static org.junit.Assert.*;

public class SessionDisplayCacheTest {
    @Rule public TemporaryFolder temporary = new TemporaryFolder();
    @Test public void isolatesSessionsAndHashesNamesIncludingPathCharacters() throws Exception {
        File root = temporary.newFolder(); SessionDisplayCache cache = new SessionDisplayCache(root);
        long now = System.currentTimeMillis();
        cache.write("../session-a", "中文消息", now); cache.write("session-b", "second", now);
        assertEquals("中文消息", cache.read("../session-a", now)); assertEquals("second", cache.read("session-b", now));
        for (String name : root.list()) assertTrue(name.matches("[0-9a-f]{64}\\.json"));
        cache.remove("../session-a"); assertNull(cache.read("../session-a", now)); assertEquals("second", cache.read("session-b", now));
    }
    @Test public void replacesAtomicallyAndRejectsUtf8ByteOverflow() throws Exception {
        File root = temporary.newFolder(); SessionDisplayCache cache = new SessionDisplayCache(root);
        long now = System.currentTimeMillis(); cache.write("a", "old", now); cache.write("a", "new", now);
        assertEquals("new", cache.read("a", now));
        String huge = new String(new char[800000]).replace('\0', '中');
        try { cache.write("a", huge, now); fail("oversized write accepted"); } catch (java.io.IOException expected) {}
        assertEquals("new", cache.read("a", now)); assertEquals(1, root.list().length);
    }
    @Test public void expiresOldFilesAndEvictsOldestWhenTotalExceedsLimit() throws Exception {
        File root = temporary.newFolder(); SessionDisplayCache cache = new SessionDisplayCache(root);
        long now = System.currentTimeMillis(); cache.write("expired", "old", now - SessionDisplayCache.MAX_AGE - 1);
        assertNull(cache.read("expired", now));
        String data = new String(new char[SessionDisplayCache.FILE_LIMIT]).replace('\0', 'x');
        for (int i=0;i<11;i++) cache.write("s"+i,data,now+i);
        assertNull(cache.read("s0", now+12)); assertEquals(data,cache.read("s10",now+12));
        assertEquals(10,root.list().length);
        cache.clear(); assertEquals(0,root.list().length);
    }
    @Test public void rejectsCorruptUtf8AndCleansInterruptedTemporaryWrites() throws Exception {
        File root = temporary.newFolder(); SessionDisplayCache cache = new SessionDisplayCache(root);
        long now=System.currentTimeMillis(); cache.write("a","valid",now);
        File file=root.listFiles()[0]; Files.write(file.toPath(),new byte[]{(byte)0xff});
        try { cache.read("a",now); fail("invalid UTF8 accepted"); } catch(java.nio.charset.CharacterCodingException expected) {}
        Files.write(new File(root,".pending-crash.tmp").toPath(),"partial".getBytes(StandardCharsets.UTF_8));
        cache.read("missing",now); assertFalse(new File(root,".pending-crash.tmp").exists());
    }
}
