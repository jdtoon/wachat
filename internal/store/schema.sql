-- Canonical wachat schema. See CLAUDE.md §7 for rules.
-- This file is the source of truth; Open() applies it on every connection.

-- Performance pragmas. WAL lets a reader run concurrently with a writer
-- (we have one writer goroutine and the UI as reader). synchronous=NORMAL
-- is safe with WAL and noticeably faster than FULL on commit-heavy
-- workloads like message ingestion.
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;

-- Messages: one row per WhatsApp message we have seen. wa_id is the dedup
-- key (whatsmeow can redeliver on reconnect). media_path stores a relative
-- path to a file on disk; the bytes are never stored in the DB (§7).
CREATE TABLE IF NOT EXISTS messages (
    id            INTEGER PRIMARY KEY,
    wa_id         TEXT UNIQUE,
    chat_jid      TEXT NOT NULL,
    sender_jid    TEXT,
    ts            INTEGER NOT NULL,     -- unix millis
    body          TEXT,
    media_path    TEXT,                 -- NULL for text-only
    media_type    TEXT,                 -- image/video/audio/doc, NULL for text
    status        TEXT NOT NULL DEFAULT 'sent', -- pending|sent|delivered|read|played
    quoted_waid   TEXT,                 -- wa_id of the message this one replies to
    quoted_body   TEXT,                 -- cached snippet of the quoted body
    quoted_sender TEXT,                 -- JID of the quoted message's sender
    edited        INTEGER NOT NULL DEFAULT 0,  -- 1 once the sender edited
    revoked       INTEGER NOT NULL DEFAULT 0   -- 1 once delete-for-everyone
);

-- Keyset paging hot path: WHERE chat_jid=? AND ts<? ORDER BY ts DESC LIMIT N.
-- Compound (chat_jid, ts DESC) covers this perfectly — no separate sort.
CREATE INDEX IF NOT EXISTS idx_chat_ts ON messages(chat_jid, ts DESC);

-- Chats: lightweight per-conversation metadata. last_ts and unread are
-- maintained by store.UpsertChat() on inserts so the chat list can render
-- without scanning messages.
CREATE TABLE IF NOT EXISTS chats (
    jid     TEXT PRIMARY KEY,
    name    TEXT,
    last_ts INTEGER,
    unread  INTEGER NOT NULL DEFAULT 0
);

-- Full-text search over message bodies. content='messages' makes this an
-- "external content" FTS5 table — it does NOT store the body twice; just
-- the index. The triggers below keep the index in sync with messages.
--
-- unicode61 tokenizer handles diacritics + case folding sensibly; the
-- 'remove_diacritics 2' option means "ç" matches "c", which is what most
-- users expect from a chat search.
CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
    body,
    content='messages',
    content_rowid='id',
    tokenize='unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages
BEGIN
    INSERT INTO messages_fts(rowid, body) VALUES (new.id, COALESCE(new.body, ''));
END;
CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages
BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, body) VALUES ('delete', old.id, COALESCE(old.body, ''));
END;
CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE ON messages
BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, body) VALUES ('delete', old.id, COALESCE(old.body, ''));
    INSERT INTO messages_fts(rowid, body) VALUES (new.id, COALESCE(new.body, ''));
END;

-- Settings: tiny key/value table for UI preferences (theme, density,
-- future sidebar width). Owned by store.GetSetting / SetSetting.
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
