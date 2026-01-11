# Screaming Frog Benzeri SEO Crawler Yazılımı — Roadmap (Uçtan Uca)

> Amaç: Screaming Frog mantığında çalışan; web sitelerini tarayan, SEO/teknik metrikleri çıkaran, raporlayan ve ölçeklenebilir bir masaüstü/servis yazılımı geliştirmek.
> Bu roadmap "çekirdek crawler → veri modeli → arayüz → modüller → raporlar → entegrasyonlar → ölçek & kalite" sırasıyla ilerler.

---

## 0) Ürün Tanımı ve Mimari Kararı (1–3 gün)
### 0.1 Hedef kapsam
- **Temel kullanım:** Bir domain gir → crawl et → "Internal / External / Response Codes / URL / Page Titles…" gibi sekmelerde sonuçları gör → filtrele → export al.
- **Desteklenen crawl modları:**
  - **Spider Mode:** Siteyi linklerden keşfederek tarama
  - **List Mode:** URL listesini içe aktar, sadece onları crawl et
  - **Sitemap Mode:** sitemap.xml üzerinden crawl
  - (Opsiyonel) **SERP/Backlink listeleri** gibi özel listeler

### 0.2 Mimari
- **Crawler motoru (core)**: URL frontier + fetch + parse + normalize + dedupe + storage pipeline
- **Parser katmanı**: HTML/HTTP header/robots meta/link extraction/structured data extractor
- **Analyzer modülleri**: her sekme (titles, meta, canonicals…) bir modül
- **UI**: tablo-sekme yapısı + filtre + arama + export + alt panel "URL Details"
- **Storage**:
  - MVP: embedded DB (SQLite) + dosya cache
  - Sonra: pluggable storage (SQLite/Postgres)
- **Render**:
  - Başlangıçta "raw HTML"
  - Sonra "JS rendering (headless Chromium)"

### 0.3 Done kriterleri
- Mimari diyagram + modül listesi + veri şeması taslağı + ilk sprint backlog.

---

## 1) Crawl Çekirdeği (MVP) — "İşin Motoru" (1–2 hafta)
### 1.1 URL Frontier & Scheduler
- Queue (BFS/DFS seçeneği)
- Depth limit (max depth)
- Max URL (limit)
- Crawl speed (requests/sec), concurrency
- Per-host politeness (crawl-delay), rate limit
- Retry/backoff + timeout
- Canonical/redirect sonrası URL yönetimi (takip seçenekleri)

### 1.2 URL Normalization & De-duplication
- Fragment (#) temizleme
- Trailing slash normalize
- www/non-www seçenekleri
- http↔https canonicalization opsiyonu
- Query param yönetimi:
  - keep/remove
  - ignore tracking params (utm, gclid vb.) listesi
- URL hashing ile "visited" kontrolü

### 1.3 Fetch Layer
- HTTP client
- Redirect takip:
  - max redirect
  - zincir kaydı
- Header capture (Server, Cache-Control, Content-Type, Content-Length…)
- Compression (gzip/br)
- TLS/SSL hata sınıflaması

### 1.4 Robots.txt + Meta Robots (temel)
- robots.txt parse + allow/disallow
- **User-agent** seçimi
- `noindex/nofollow/noarchive/nosnippet` meta robots tespiti
- X-Robots-Tag header tespiti

### 1.5 Parse (temel HTML)
- `<title>`, meta description, canonical, headings
- Link extraction:
  - `<a href>`, `<img src>`, `<link href>`, `<script src>`, iframe, video/audio
- Internal/External sınıflandırma

### 1.6 MVP çıktıları (ilk "Screaming Frog benzeri tablo")
Sekmeler (ilk sürüm):
- **Internal**
- **External**
- **Response Codes**
- **URL**

Her satır için minimum kolonlar:
- Address (URL)
- Content Type
- Status Code
- Status (OK/Redirect/Blocked/Error)
- Crawl Depth
- Inlinks Count (temel)

**Done kriteri:** 1 domain crawl edilebiliyor, sonuçlar tabloya düşüyor, export alınabiliyor.

---

## 2) Veri Modeli ve Storage (3–5 gün)
### 2.1 Temel tablolar
- `urls` (id, url, normalized_url, discovered_from, depth, first_seen, last_seen)
- `fetches` (url_id, status_code, content_type, content_length, response_time, final_url_id, redirect_chain_id…)
- `html_features` (title, meta_desc, canonical, h1, h2, word_count…)
- `links` (from_url_id, to_url, to_url_id?, link_type, rel, anchor_text, nofollow…)
- `resources` (img/css/js/iframe; size; status; mime)
- `issues` (url_id, issue_code, severity, details_json)

### 2.2 Incremental write (streaming)
- Crawl devam ederken UI güncellenebilir
- "Pause/Resume" için state saklama

**Done kriteri:** 10k+ URL crawl'da RAM şişmeden stabil.

---

## 3) UI/UX — Screaming Frog Benzeri Arayüz (1–2 hafta)
### 3.1 Tabbed Grid Yapısı
Görseldeki ana sekmeler (minimum):
- Internal, External, Response Codes, URL
- Page Titles, Meta Description, Meta Keywords
- H1, H2
- Content
- Images
- Canonicals
- Pagination
- Directives
- Hreflang
- JavaScript
- Links
- AMP
- Structured Data
- Sitemaps
- PageSpeed
- Mobile
- Accessibility
- Custom Search
- Custom Extraction

### 3.2 Filtre & Arama
- Sekme bazlı quick filter (All, Missing, Duplicate, Over/Under length vb.)
- Global search input (URL/Title/Meta içinde arama)
- Çoklu kolon sort
- "Export current view" + "Export all"

### 3.3 Alt Panel (URL Details)
Alt sekmeler:
- URL Details (summary)
- Inlinks (from nereden geliyor)
- Outlinks (nereye gidiyor)
- Image Details
- Resources (JS/CSS vb.)
- Rendered Page (ileride)
- View Source
- HTTP Headers
- (Opsiyonel) Chrome Console Log (ileride)

**Done kriteri:** Kullanıcı SF'ye benzer şekilde veriyi gezebiliyor, filtreleyebiliyor, export alabiliyor.

---

## 4) Modül Modül Özellik Geliştirme (Screaming Frog Sekmeleri) (2–6 hafta)

Aşağıdaki her modül için ortak çıktı:
- Kolon seti
- Filtre seti (Missing/Duplicate/Too Long vb.)
- Issue üretimi (issues tablosu)
- Bulk export (opsiyonel)

---

### 4.1 Response Codes Modülü
**Amaç:** Tüm URL'leri status code'a göre sınıflandırmak.
- 2xx, 3xx, 4xx, 5xx, blocked, timeout
- Redirect chain analizi:
  - chain length
  - final destination
  - 302/307 geçici yönlendirme
  - loop tespiti
- Soft 404 heuristics (opsiyonel)

**Filtreler:**
- 3xx only, 4xx only, 5xx only
- Redirect Chains (len > 1)
- Redirect Loops

---

### 4.2 Internal / External Modülü
**Amaç:** Link keşfi ve domain sınırları.
- Internal tanımı: aynı host / aynı registrable domain (seçenekli)
- Subdomain dahil et / etme
- CDN/asset host'larını internal say (kural seti)

**Kolonlar (öneri):**
- URL
- Depth
- Inlinks
- Outlinks
- Indexability (robots/meta)
- Canonical target

---

### 4.3 URL Modülü (URL Health)
- URL length
- Param sayısı
- Uppercase/space/encoded char tespiti
- Duplicate URL (normalize sonrası aynı şeye düşen) raporu
- Trailing slash tutarsızlığı
- Mixed protocol (http/https)

---

### 4.4 Page Titles Modülü
- Title var/yok
- Title length (chars & pixels opsiyonel)
- Duplicate titles
- Multiple titles (edge case)
- Title same as H1 (opsiyonel check)

**Filtreler:**
- Missing, Duplicate, Too Long, Too Short

---

### 4.5 Meta Description Modülü
- Meta description var/yok
- Length
- Duplicate meta descriptions

---

### 4.6 Meta Keywords (opsiyonel ama SF'de var)
- Varsa parse et ve raporla (SEO değeri düşük ama parity için)

---

### 4.7 H1 / H2 Modülü
- H1 count (0,1,>1)
- H1 text
- H2 count + ilk N H2 text
- Duplicate H1'ler
- H1 çok uzun/kısa

---

### 4.8 Content Modülü
- Word count (visible text)
- Thin content tespiti (threshold)
- Duplicate content (hash):
  - exact duplicate (MD5/SHA of cleaned text)
  - near duplicate (shingling + simhash) (ileri seviye)
- Readability (opsiyonel)
- Pagination content footprint

---

### 4.9 Images Modülü
- `<img>` sayısı
- Missing alt
- Large images (size threshold)
- Broken images (status code)
- Image dimensions (opsiyonel: HEAD/GET + metadata)
- Lazyload pattern tespiti (`data-src`, `loading="lazy"`)

---

### 4.10 Canonicals Modülü
- Canonical var/yok
- Self-referencing canonical?
- Canonical pointing to 3xx/4xx
- Canonical chain
- Canonical mismatch:
  - canonical farklı URL ama içerik aynı/benzer
- Multiple canonicals

---

### 4.11 Pagination Modülü
- `rel="next"` / `rel="prev"` (tarihi ama parity için)
- Pagination param patterns (?page=, /page/2/)
- Paginated series tespiti
- Canonical + pagination uyumu

---

### 4.12 Directives Modülü
**robots/meta directives**
- Meta robots parse (`noindex`, `nofollow`, `noarchive`, `max-snippet`, `max-image-preview`, `max-video-preview`)
- X-Robots-Tag header parse
- Canonical + noindex kombinasyonları (riskli durumlar)
- `nofollow` page-level ve link-level ayrımı (rel)

---

### 4.13 Hreflang Modülü
- `<link rel="alternate" hreflang="…">` çıkarımı
- Return tag (karşılıklı doğrulama)
- Self-reference hreflang
- x-default var mı
- Hreflang target 3xx/4xx
- Dil/bölge kod validasyonu

---

### 4.14 JavaScript Modülü (Kaynak + Render)
İki katman önerisi:
1) **Basit (MVP):** JS dosyalarını listele (script src), status, size
2) **İleri (Render):** Headless Chromium ile render edip:
   - Rendered HTML
   - Render sonrası link keşfi (SPA)
   - JS console errors capture
   - Network waterfall (opsiyonel)

---

### 4.15 Links Modülü
- Inlinks/Outlinks detayı:
  - Anchor text
  - Follow/nofollow/sponsored/ugc
  - Link position (header/footer/body) (heuristic)
- Broken links (to 4xx/5xx)
- Redirecting links (to 3xx)
- Orphan pages:
  - sitemap'te var ama internal link yok
  - list mode'da var ama internal yok

---

### 4.16 AMP Modülü
- AMP HTML tespiti (`<link rel="amphtml">`)
- AMP canonical relationship doğrulama
- AMP URL status & indexability

---

### 4.17 Structured Data Modülü
- JSON-LD parse
- Microdata/RDFa tespiti
- Schema types listesi (Article, BreadcrumbList, FAQPage…)
- Temel doğrulamalar:
  - JSON parse error
  - required fields "lite checks" (tam spec doğrulaması ileri seviye)
- Rich result uygunluk raporu (opsiyonel)

---

### 4.18 Sitemaps Modülü
- sitemap.xml fetch + parse (index + urlset)
- sitemap'teki URL'leri internal crawl ile karşılaştır:
  - sitemap only (orphan)
  - crawled but not in sitemap
- lastmod parse
- sitemap status/format hataları

---

### 4.19 PageSpeed Modülü (Entegrasyon)
- PSI (PageSpeed Insights) API ile:
  - LCP, CLS, INP, TTFB, FCP
  - Lab/Field ayrımı (mevcutsa)
- URL başına rate-limit + caching
- "sample mode" (her URL için değil; seçili/filtreli)

---

### 4.20 Mobile Modülü
- Mobile friendly heuristics:
  - viewport meta var mı
  - responsive check (CSS media queries var mı - heuristic)
- (Opsiyonel) Mobile render profile ile crawl

---

### 4.21 Accessibility Modülü
- Alt missing (images)
- Form label missing (basic)
- Title attribute misuse (opsiyonel)
- Heading hierarchy sorunları (H1→H3 atlama vb.)
- ARIA basic checks (ileri seviye)

---

### 4.22 Custom Search Modülü
**Amaç:** Kullanıcının belirlediği pattern/regex/selector ile sayfa içinde arama.
- Text search (case-sensitive, regex)
- HTML source search
- CSS selector ile element var mı / kaç tane
- Sonuç kolonları:
  - matched? (true/false)
  - count
  - sample snippet

---

### 4.23 Custom Extraction Modülü
**Amaç:** CSS/XPath/Regex ile alan çıkar.
- CSS selector / XPath
- Regex group capture
- Çoklu extraction rule set
- Export: URL + extracted fields

---

## 5) Crawl Konfigürasyonları (Screaming Frog "Configuration" parity) (1–3 hafta)
### 5.1 Include/Exclude
- Include regex
- Exclude regex
- Crawl outside of start folder (seçenek)

### 5.2 Limits
- Max depth, max URLs, max query parameters
- Max response size
- Crawl duration limit

### 5.3 Rendering seçenekleri
- HTML only
- JS rendering (Chromium)
- Render timeout
- Wait for network idle / DOMContentLoaded

### 5.4 Authentication
- Basic auth
- Cookie jar / session
- Header injection (Authorization)
- (Opsiyonel) Form login macro (ileri)

### 5.5 Robots & nofollow handling
- Respect robots on/off
- Respect nofollow links on/off
- Crawl canonical URLs on/off

---

## 6) Raporlama, Bulk Export ve "Reports" Menüsü (1–2 hafta)
### 6.1 Hazır raporlar
- All redirects
- Redirect chains
- Client errors (4xx)
- Server errors (5xx)
- Canonical errors
- Missing titles/meta/h1
- Duplicate titles/meta
- Missing alt
- Orphan URLs (sitemap/list kaynaklı)
- Non-indexable pages
- Pages with no internal inlinks

### 6.2 Export formatları
- CSV, XLSX
- JSON (API amaçlı)
- "Export current filters" / "Export all"

---

## 7) Visualisations & Crawl Analysis (1–3 hafta)
### 7.1 Site Structure
- URL path tree (folder bazlı)
- Depth distribution grafikleri
- Status code distribution

### 7.2 Force-Directed Graph / Crawl Tree
- Node: URL
- Edge: link
- Filtre: internal only, status-based

### 7.3 Segmentler
- Segment by:
  - content type
  - status code
  - indexability
  - templates (URL pattern)

---

## 8) Entegrasyonlar (İleri Seviye) (2–6 hafta)
### 8.1 Google Search Console
- Performance verisi ile URL'leri zenginleştir:
  - clicks, impressions, ctr, position
- Index coverage / pages report eşleştirme

### 8.2 Google Analytics (GA4)
- Landing page sessions, engagement, conversions

### 8.3 PageSpeed API
- Zaten 4.19'da var, burada "bulk orchestrator" geliştir.

### 8.4 Log File (opsiyonel ürün/modül)
- Server logs parse
- Crawl data ile eşleştirme:
  - Googlebot hit frequency
  - orphan + actually crawled URLs

---

## 9) Performans, Ölçek ve Stabilite (sürekli)
### 9.1 Büyük crawl optimizasyonu
- Streaming parser
- Backpressure
- Disk cache
- Memory cap

### 9.2 Hata toleransı
- Partial crawl recovery
- Checkpoint
- Crash sonrası resume

### 9.3 Test stratejisi
- Unit: URL normalization, robots parse, parser
- Integration: small test sites
- Regression snapshots: export karşılaştırma

---

## 10) Güvenlik ve Uyumluluk (sürekli)
- User-agent ve crawl politeness
- PII yakalama riskleri (custom extraction)
- Rate limiting + "do no harm" defaults
- HTTPS/TLS hata raporları

---

## 11) Sürümleme Planı (Önerilen)
### v0.1 — MVP Crawler
- Internal/External/Response Codes/URL + export + temel UI

### v0.2 — On-page SEO
- Titles/Meta/H1/H2/Canonicals/Images + issue raporları

### v0.3 — Links & Directives
- Inlinks/Outlinks detay, noindex/nofollow, redirect chains

### v0.4 — Sitemap + Hreflang
- Sitemap audit, hreflang validation

### v0.5 — Custom Search/Extraction
- Kullanıcı tanımlı arama ve extraction rule engine

### v0.6 — JS Rendering
- Headless render, rendered HTML, console log, SPA link discovery

### v0.7 — Structured Data + PSI
- Schema parse + PageSpeed metrikleri (seçmeli/bulk)

### v1.0 — "Screaming Frog parity + polishing"
- Visualisations, segments, report pack, stabilite, lisanslama

---

## 12) "Screaming Frog'ta olup genelde unutulan" Feature Checklist
- List Mode (CSV/Excel import) ✅
- Sitemap Mode ✅
- Crawl comparison (2 crawl diff) (opsiyonel)
- Scheduling (CLI + cron) (opsiyonel)
- Regex-based filters ✅
- Duplicate cluster view (title/meta/content) ✅
- Near-duplicate (simhash) (opsiyonel)
- Export templating (kolon seçimi) ✅
- Rendered page viewer ✅
- HTTP header rules (custom checks) (opsiyonel)

---

## 13) İlk Sprint İçin Net To-Do (Başlangıç)
1) URL normalization + queue + fetch + parse (title/meta/h1/link)
2) Internal/External/Response Codes sekmeleri
3) Basit UI tablo + filtre + export CSV
4) URL Details panel: inlinks/outlinks + headers
5) Robots.txt temel uyum

**Sprint Done:** 1–5k URL'lik siteyi stabil crawl edip export alabiliyor.

---
