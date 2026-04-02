CREATE TABLE IF NOT EXISTS vendors (
                                       id TEXT PRIMARY KEY,
                                       name TEXT NOT NULL,
                                       endpoint_url TEXT NOT NULL,
                                       method TEXT NOT NULL DEFAULT 'POST',
                                       headers TEXT,
                                       body_template TEXT,
                                       timeout_ms INTEGER NOT NULL DEFAULT 5000,
                                       skip_tls_verify BOOLEAN NOT NULL DEFAULT 0,
                                       ca_cert TEXT,
                                       created_at DATETIME NOT NULL,
                                       updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS notifications (
                                             id INTEGER PRIMARY KEY AUTOINCREMENT,
                                             out_biz_no TEXT NOT NULL,
                                             vendor_id TEXT NOT NULL,
                                             event_type TEXT NOT NULL,
                                             payload TEXT NOT NULL,
                                             status TEXT NOT NULL DEFAULT 'pending',
                                             attempts INTEGER NOT NULL DEFAULT 0,
                                             last_attempt_at DATETIME,
                                             next_attempt_at DATETIME NOT NULL,
                                             error_message TEXT,
                                             created_at DATETIME NOT NULL,
                                             updated_at DATETIME NOT NULL,
                                             UNIQUE(out_biz_no, vendor_id)
);

CREATE INDEX IF NOT EXISTS idx_notifications_status_next ON notifications(status, next_attempt_at);
CREATE INDEX IF NOT EXISTS idx_notifications_vendor ON notifications(vendor_id);
