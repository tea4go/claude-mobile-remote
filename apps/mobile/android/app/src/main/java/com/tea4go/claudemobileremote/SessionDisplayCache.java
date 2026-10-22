package com.tea4go.claudemobileremote;

import java.io.File;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.StandardCopyOption;
import java.nio.file.AtomicMoveNotSupportedException;
import java.security.MessageDigest;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;

/** Display files only. The caller serializes access; credentials remain in secure storage. */
final class SessionDisplayCache {
    static final int FILE_LIMIT = 2 * 1024 * 1024;
    static final long TOTAL_LIMIT = 20L * 1024 * 1024;
    static final long MAX_AGE = 30L * 24 * 60 * 60 * 1000;
    private final File directory;
    SessionDisplayCache(File directory) { this.directory = directory; }
    private File file(String sessionId) throws Exception {
        if (sessionId == null || sessionId.isEmpty() || sessionId.getBytes(StandardCharsets.UTF_8).length > 1024) throw new IOException("invalid session");
        StringBuilder name = new StringBuilder();
        for (byte value : MessageDigest.getInstance("SHA-256").digest(sessionId.getBytes(StandardCharsets.UTF_8))) name.append(String.format("%02x", value & 255));
        return new File(directory, name + ".json");
    }
    String read(String sessionId, long now) throws Exception {
        prune(now);
        File file = file(sessionId);
        if (!file.exists()) return null;
        return StandardCharsets.UTF_8.newDecoder().decode(java.nio.ByteBuffer.wrap(Files.readAllBytes(file.toPath()))).toString();
    }
    void write(String sessionId, String data, long now) throws Exception {
        File destination = file(sessionId);
        byte[] bytes = data.getBytes(StandardCharsets.UTF_8);
        if (bytes.length > FILE_LIMIT) throw new IOException("cache too large");
        if (!directory.isDirectory() && !directory.mkdirs()) throw new IOException("cache unavailable");
        File temporary = File.createTempFile(".pending-", ".tmp", directory);
        try {
            Files.write(temporary.toPath(), bytes);
            try { Files.move(temporary.toPath(), destination.toPath(), StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING); }
            catch (AtomicMoveNotSupportedException error) { Files.move(temporary.toPath(), destination.toPath(), StandardCopyOption.REPLACE_EXISTING); }
            if (!destination.setLastModified(now)) throw new IOException("cache timestamp unavailable");
        } finally { Files.deleteIfExists(temporary.toPath()); }
        prune(now);
    }
    void remove(String sessionId) throws Exception { Files.deleteIfExists(file(sessionId).toPath()); }
    void clear() throws IOException {
        File[] files = directory.listFiles();
        if (files != null) for (File file : files) Files.deleteIfExists(file.toPath());
    }
    private void prune(long now) throws IOException {
        File[] files = directory.listFiles();
        if (files == null) return;
        List<File> retained = new ArrayList<>();
        long total = 0;
        for (File file : files) {
            if (!file.getName().endsWith(".json")) {
                if (file.getName().startsWith(".pending-")) Files.deleteIfExists(file.toPath());
                continue;
            }
            if (file.length() > FILE_LIMIT || now - file.lastModified() > MAX_AGE) Files.deleteIfExists(file.toPath());
            else { retained.add(file); total += file.length(); }
        }
        retained.sort(Comparator.comparingLong(File::lastModified));
        for (File file : retained) {
            if (total <= TOTAL_LIMIT) break;
            total -= file.length(); Files.deleteIfExists(file.toPath());
        }
    }
}
