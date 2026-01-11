package storage

// Schema contains SQL statements to create database tables.
const Schema = `
-- URLs table: stores all discovered URLs
CREATE TABLE IF NOT EXISTS urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL,
    normalized_url TEXT NOT NULL UNIQUE,
    host TEXT NOT NULL,
    path TEXT,
    discovered_from INTEGER REFERENCES urls(id),
    depth INTEGER DEFAULT 0,
    first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
    crawl_status TEXT DEFAULT 'pending',
    is_internal BOOLEAN DEFAULT 1,
    in_sitemap BOOLEAN DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_urls_normalized ON urls(normalized_url);
CREATE INDEX IF NOT EXISTS idx_urls_host ON urls(host);
CREATE INDEX IF NOT EXISTS idx_urls_crawl_status ON urls(crawl_status);
CREATE INDEX IF NOT EXISTS idx_urls_is_internal ON urls(is_internal);

-- Fetches table: stores HTTP response data for each crawl
CREATE TABLE IF NOT EXISTS fetches (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url_id INTEGER NOT NULL REFERENCES urls(id),
    status_code INTEGER,
    status TEXT,
    content_type TEXT,
    content_length INTEGER,
    response_time_ms INTEGER,
    ttfb_ms INTEGER,
    final_url_id INTEGER REFERENCES urls(id),
    redirect_chain_id INTEGER REFERENCES redirect_chains(id),
    fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    headers_json TEXT,
    tls_version TEXT,
    tls_issuer TEXT,
    tls_expiry TEXT
);

CREATE INDEX IF NOT EXISTS idx_fetches_url_id ON fetches(url_id);
CREATE INDEX IF NOT EXISTS idx_fetches_status_code ON fetches(status_code);
CREATE INDEX IF NOT EXISTS idx_fetches_fetched_at ON fetches(fetched_at);

-- HTML Features table: stores SEO-relevant HTML data
CREATE TABLE IF NOT EXISTS html_features (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url_id INTEGER NOT NULL UNIQUE REFERENCES urls(id),
    title TEXT,
    title_length INTEGER DEFAULT 0,
    meta_description TEXT,
    meta_desc_length INTEGER DEFAULT 0,
    meta_keywords TEXT,
    meta_robots TEXT,
    canonical TEXT,
    canonical_url_id INTEGER REFERENCES urls(id),
    h1_count INTEGER DEFAULT 0,
    h1_first TEXT,
    h1_all TEXT,
    h2_count INTEGER DEFAULT 0,
    h2_all TEXT,
    word_count INTEGER DEFAULT 0,
    content_hash TEXT,
    language TEXT,
    hreflangs TEXT,
    og_title TEXT,
    og_description TEXT,
    og_image TEXT,
    is_indexable BOOLEAN DEFAULT 1,
    index_status TEXT
);

CREATE INDEX IF NOT EXISTS idx_html_features_url_id ON html_features(url_id);
CREATE INDEX IF NOT EXISTS idx_html_features_content_hash ON html_features(content_hash);
CREATE INDEX IF NOT EXISTS idx_html_features_title ON html_features(title);

-- Links table: stores link relationships
CREATE TABLE IF NOT EXISTS links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_url_id INTEGER NOT NULL REFERENCES urls(id),
    to_url TEXT NOT NULL,
    to_url_id INTEGER REFERENCES urls(id),
    anchor_text TEXT,
    link_type TEXT DEFAULT 'a',
    rel TEXT,
    is_internal BOOLEAN DEFAULT 0,
    is_follow BOOLEAN DEFAULT 1,
    position TEXT
);

CREATE INDEX IF NOT EXISTS idx_links_from_url ON links(from_url_id);
CREATE INDEX IF NOT EXISTS idx_links_to_url ON links(to_url_id);
CREATE INDEX IF NOT EXISTS idx_links_is_internal ON links(is_internal);

-- Resources table: stores external resources (images, JS, CSS)
CREATE TABLE IF NOT EXISTS resources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL UNIQUE,
    url_id INTEGER REFERENCES urls(id),
    resource_type TEXT NOT NULL,
    mime_type TEXT,
    status_code INTEGER,
    size INTEGER,
    first_seen_on INTEGER REFERENCES urls(id),
    alt TEXT,
    width INTEGER,
    height INTEGER,
    is_async BOOLEAN,
    is_defer BOOLEAN
);

CREATE INDEX IF NOT EXISTS idx_resources_url ON resources(url);
CREATE INDEX IF NOT EXISTS idx_resources_type ON resources(resource_type);
CREATE INDEX IF NOT EXISTS idx_resources_status ON resources(status_code);

-- Page Resources junction table
CREATE TABLE IF NOT EXISTS page_resources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url_id INTEGER NOT NULL REFERENCES urls(id),
    resource_id INTEGER NOT NULL REFERENCES resources(id),
    UNIQUE(url_id, resource_id)
);

CREATE INDEX IF NOT EXISTS idx_page_resources_url ON page_resources(url_id);
CREATE INDEX IF NOT EXISTS idx_page_resources_resource ON page_resources(resource_id);

-- Issues table: stores SEO issues
CREATE TABLE IF NOT EXISTS issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url_id INTEGER NOT NULL REFERENCES urls(id),
    issue_code TEXT NOT NULL,
    issue_type TEXT NOT NULL,
    severity TEXT NOT NULL,
    category TEXT,
    message TEXT,
    details TEXT,
    detected_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_issues_url_id ON issues(url_id);
CREATE INDEX IF NOT EXISTS idx_issues_code ON issues(issue_code);
CREATE INDEX IF NOT EXISTS idx_issues_severity ON issues(severity);
CREATE INDEX IF NOT EXISTS idx_issues_category ON issues(category);

-- Redirect Chains table
CREATE TABLE IF NOT EXISTS redirect_chains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    start_url TEXT NOT NULL,
    final_url TEXT NOT NULL,
    chain_json TEXT NOT NULL,
    length INTEGER NOT NULL,
    has_loop BOOLEAN DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_redirect_chains_start ON redirect_chains(start_url);

-- Crawl Sessions table: stores crawl session metadata
CREATE TABLE IF NOT EXISTS crawl_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    start_url TEXT NOT NULL,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    status TEXT DEFAULT 'running',
    total_urls INTEGER DEFAULT 0,
    crawled_urls INTEGER DEFAULT 0,
    failed_urls INTEGER DEFAULT 0,
    config_json TEXT,
    last_checkpoint DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Sitemaps table
CREATE TABLE IF NOT EXISTS sitemaps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL UNIQUE,
    type TEXT,
    url_count INTEGER DEFAULT 0,
    last_fetched DATETIME,
    status_code INTEGER,
    error_msg TEXT
);

-- Sitemap URLs junction table
CREATE TABLE IF NOT EXISTS sitemap_urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sitemap_id INTEGER NOT NULL REFERENCES sitemaps(id),
    url_id INTEGER NOT NULL REFERENCES urls(id),
    lastmod DATETIME,
    changefreq TEXT,
    priority REAL,
    UNIQUE(sitemap_id, url_id)
);

CREATE INDEX IF NOT EXISTS idx_sitemap_urls_sitemap ON sitemap_urls(sitemap_id);
CREATE INDEX IF NOT EXISTS idx_sitemap_urls_url ON sitemap_urls(url_id);

-- Crawl Queue table: for pause/resume support
CREATE TABLE IF NOT EXISTS crawl_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL REFERENCES crawl_sessions(id),
    url_id INTEGER NOT NULL REFERENCES urls(id),
    priority INTEGER DEFAULT 0,
    scheduled_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    retry_count INTEGER DEFAULT 0,
    status TEXT DEFAULT 'pending'
);

CREATE INDEX IF NOT EXISTS idx_crawl_queue_session ON crawl_queue(session_id);
CREATE INDEX IF NOT EXISTS idx_crawl_queue_status ON crawl_queue(status);
CREATE INDEX IF NOT EXISTS idx_crawl_queue_priority ON crawl_queue(priority);
`

// ViewsSchema contains SQL for useful views
const ViewsSchema = `
-- View: Internal pages with their fetch status
CREATE VIEW IF NOT EXISTS v_internal_pages AS
SELECT
    u.id,
    u.url,
    u.depth,
    u.crawl_status,
    f.status_code,
    f.content_type,
    f.response_time_ms,
    h.title,
    h.meta_description,
    h.h1_first,
    h.word_count,
    h.is_indexable,
    (SELECT COUNT(*) FROM links WHERE to_url_id = u.id) as inlinks_count,
    (SELECT COUNT(*) FROM links WHERE from_url_id = u.id) as outlinks_count
FROM urls u
LEFT JOIN fetches f ON f.url_id = u.id
LEFT JOIN html_features h ON h.url_id = u.id
WHERE u.is_internal = 1;

-- View: External links
CREATE VIEW IF NOT EXISTS v_external_links AS
SELECT
    u.url as from_url,
    l.to_url,
    l.anchor_text,
    l.rel,
    l.is_follow
FROM links l
JOIN urls u ON l.from_url_id = u.id
WHERE l.is_internal = 0;

-- View: Response codes summary
CREATE VIEW IF NOT EXISTS v_response_codes AS
SELECT
    f.status_code,
    COUNT(*) as count,
    GROUP_CONCAT(u.url) as urls
FROM fetches f
JOIN urls u ON f.url_id = u.id
GROUP BY f.status_code
ORDER BY f.status_code;

-- View: Issues summary
CREATE VIEW IF NOT EXISTS v_issues_summary AS
SELECT
    issue_code,
    severity,
    COUNT(*) as count
FROM issues
GROUP BY issue_code, severity
ORDER BY
    CASE severity
        WHEN 'critical' THEN 1
        WHEN 'high' THEN 2
        WHEN 'medium' THEN 3
        WHEN 'low' THEN 4
    END,
    count DESC;

-- View: Duplicate titles
CREATE VIEW IF NOT EXISTS v_duplicate_titles AS
SELECT
    h.title,
    COUNT(*) as count,
    GROUP_CONCAT(u.url, ' | ') as urls
FROM html_features h
JOIN urls u ON h.url_id = u.id
WHERE h.title IS NOT NULL AND h.title != ''
GROUP BY h.title
HAVING COUNT(*) > 1
ORDER BY count DESC;

-- View: Duplicate meta descriptions
CREATE VIEW IF NOT EXISTS v_duplicate_meta_desc AS
SELECT
    h.meta_description,
    COUNT(*) as count,
    GROUP_CONCAT(u.url, ' | ') as urls
FROM html_features h
JOIN urls u ON h.url_id = u.id
WHERE h.meta_description IS NOT NULL AND h.meta_description != ''
GROUP BY h.meta_description
HAVING COUNT(*) > 1
ORDER BY count DESC;

-- View: Redirect chains
CREATE VIEW IF NOT EXISTS v_redirect_chains AS
SELECT
    rc.start_url,
    rc.final_url,
    rc.length,
    rc.has_loop,
    rc.chain_json
FROM redirect_chains rc
WHERE rc.length > 1
ORDER BY rc.length DESC;

-- View: Orphan pages (no internal inlinks)
CREATE VIEW IF NOT EXISTS v_orphan_pages AS
SELECT
    u.url,
    u.depth,
    h.title
FROM urls u
LEFT JOIN html_features h ON h.url_id = u.id
WHERE u.is_internal = 1
AND u.crawl_status = 'crawled'
AND NOT EXISTS (
    SELECT 1 FROM links l
    WHERE l.to_url_id = u.id
    AND l.is_internal = 1
    AND l.from_url_id != u.id
);

-- View: Images missing alt
CREATE VIEW IF NOT EXISTS v_images_missing_alt AS
SELECT
    r.url as image_url,
    r.alt,
    u.url as found_on
FROM resources r
JOIN page_resources pr ON pr.resource_id = r.id
JOIN urls u ON pr.url_id = u.id
WHERE r.resource_type = 'image'
AND (r.alt IS NULL OR r.alt = '');
`
