# Vimango iOS Companion App - Planning Document

## Executive Summary

Building a read-only iOS companion app for vimango is **feasible and relatively straightforward**. The existing architecture already supports PostgreSQL synchronization, has well-defined data structures, and uses standard markdown for note content. The main complexity lies in Google Drive image handling.

**Recommended Approach:** Hybrid architecture with local SQLite cache plus PostgreSQL sync, with an optional lightweight API layer for image proxying.

---

## Key Design Decisions

### 1. Local SQLite vs Remote PostgreSQL

**Recommendation: Both (Cached Sync Model)**

| Approach | Pros | Cons |
|----------|------|------|
| **Remote Only** | Always fresh data, simple | No offline access, network latency, exposed credentials |
| **Local Only** | Full offline, fast | Requires manual export/import, data gets stale |
| **Hybrid (Recommended)** | Offline capable, fresh when online | Slightly more complex |

**Why Hybrid:**
- Vimango already has proven sync logic between SQLite and PostgreSQL
- You can read notes on a plane, in the subway, etc.
- Sync only happens when you open the app (battery friendly)
- The sync code patterns already exist in `synchronize.go`

### 2. Architecture Options

#### Option A: Direct PostgreSQL (Simplest Start)
```
iOS App <---> PostgreSQL Server
```
- Use PostgreSQL iOS client library (e.g., PostgresClientKit)
- Store credentials securely in Keychain
- Requires server to be accessible (VPN, SSH tunnel, or public with SSL)

**Pros:** No new backend code, leverages existing infrastructure
**Cons:** Database credentials on device, no image proxying

#### Option B: REST API Layer (Recommended for Production)
```
iOS App <---> Go API Server <---> PostgreSQL
                   |
                   v
              Google Drive (image proxy)
```
- Build lightweight Go API (reuses existing vimango DB code)
- Handles authentication, rate limiting, image proxying
- Can be deployed alongside existing infrastructure

**Pros:** Secure, handles images elegantly, can add features later
**Cons:** Requires deploying/maintaining API server

#### Option C: Export File Sync (No Server Changes)
```
Desktop vimango --export--> JSON/SQLite file
                                  |
                            iCloud/Dropbox
                                  |
                                  v
                            iOS App (import)
```
- Desktop app gets new `:export` command
- iOS app imports and stores locally
- Sync via cloud storage

**Pros:** No server needed, works fully offline
**Cons:** Manual process, potential for stale data

---

## Data Model for iOS

### Core Entities

```swift
// Note/Entry (maps to 'task' table)
struct VNote: Codable, Identifiable {
    let tid: Int
    var id: Int { tid }
    let title: String
    let note: String?          // Markdown content
    let star: Bool
    let archived: Bool
    let deleted: Bool
    let contextUUID: String
    let folderUUID: String
    let contextTitle: String   // Denormalized for display
    let folderTitle: String    // Denormalized for display
    let keywords: [String]     // Resolved keyword titles
    let added: Date
    let modified: Date
}

// Container (context, folder, or keyword)
struct VContainer: Codable, Identifiable {
    let tid: Int
    var id: Int { tid }
    let uuid: String
    let title: String
    let star: Bool
    let containerType: ContainerType

    enum ContainerType: String, Codable {
        case context, folder, keyword
    }
}
```

### SQLite Schema for iOS Cache

```sql
-- Mirrors server schema but simplified for read-only use
CREATE TABLE note (
    tid INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    note TEXT,
    star INTEGER DEFAULT 0,
    archived INTEGER DEFAULT 0,
    context_uuid TEXT,
    folder_uuid TEXT,
    added TEXT,      -- ISO8601 timestamp
    modified TEXT
);

CREATE TABLE container (
    tid INTEGER PRIMARY KEY,
    uuid TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    star INTEGER DEFAULT 0,
    container_type TEXT NOT NULL  -- 'context', 'folder', 'keyword'
);

CREATE TABLE note_keyword (
    note_tid INTEGER,
    keyword_uuid TEXT,
    PRIMARY KEY (note_tid, keyword_uuid)
);

-- Metadata for sync
CREATE TABLE sync_meta (
    key TEXT PRIMARY KEY,
    value TEXT
);
-- Store: last_sync_timestamp, server_version
```

---

## Image Handling Strategy

### The Challenge

Notes contain image references like:
```markdown
![Photo caption](gdrive:19_FwuxjvgIwxn-b4Ia75DrXRGLZeB2fe)
```

These require Google Drive API access to fetch.

### Options

#### 1. Direct Google Drive Access from iOS
- Use Google Sign-In SDK for iOS OAuth
- Use Google Drive API to download images
- Cache in app's Documents directory

**Challenge:** User needs to authorize Google Drive on iOS (separate from desktop auth)

#### 2. API Proxy (Recommended with Option B)
- API endpoint: `GET /api/image/{gdrive_id}`
- Server fetches from Google Drive using existing auth
- Returns image bytes with caching headers
- iOS treats it as normal URL

**Advantage:** No Google OAuth needed on iOS

#### 3. Pre-cache During Sync
- When syncing notes, also sync associated images
- Store image data in SQLite as BLOB or file references
- Increases sync time/bandwidth but provides offline images

### Image Parsing

Reuse the existing regex patterns from vimango:
```swift
// Swift equivalents of vimango's regex
let gdriveShortPattern = #"!\[([^\]]*)\]\((gdrive:[a-zA-Z0-9_-]+)\)"#
let gdriveFullPattern = #"!\[([^\]]*)\]\((https://drive\.google\.com/file/d/[^)]+)\)"#

func extractFileID(from url: String) -> String? {
    if url.hasPrefix("gdrive:") {
        return String(url.dropFirst(7))
    }
    // Parse full URL for /d/{id} pattern
    let pattern = #"/d/([a-zA-Z0-9_-]+)"#
    // ... regex extraction
}
```

---

## Sync Protocol

### Initial Sync (First Launch)
1. Fetch all non-deleted containers (contexts, folders, keywords)
2. Fetch all non-deleted, non-archived notes
3. Store in local SQLite
4. Record sync timestamp

### Incremental Sync (Subsequent Launches)
1. Send last sync timestamp to server
2. Receive only modified items since that timestamp
3. Upsert into local SQLite
4. Handle deleted items (mark as deleted locally)

### API Endpoints Needed (Option B)

```
GET /api/sync?since={timestamp}
Response:
{
    "timestamp": "2024-01-15T10:30:00Z",
    "contexts": [...],
    "folders": [...],
    "keywords": [...],
    "notes": [...],
    "deleted_note_tids": [...]
}

GET /api/notes
GET /api/notes/{tid}
GET /api/image/{gdrive_id}
```

### SQL Queries (Option A - Direct PostgreSQL)

```sql
-- Incremental sync: get modified notes
SELECT t.tid, t.title, t.note, t.star, t.archived, t.deleted,
       t.context_uuid, t.folder_uuid, t.added, t.modified,
       c.title AS context_title, f.title AS folder_title
FROM task t
JOIN context c ON c.uuid = t.context_uuid
JOIN folder f ON f.uuid = t.folder_uuid
WHERE t.modified > $1  -- last_sync_timestamp
ORDER BY t.modified;

-- Get keywords for notes
SELECT tk.task_tid, k.title
FROM task_keyword tk
JOIN keyword k ON k.uuid = tk.keyword_uuid
WHERE tk.task_tid = ANY($1);  -- array of note tids
```

---

## Technology Stack Recommendation

### iOS App
- **Language:** Swift
- **UI Framework:** SwiftUI (modern, declarative)
- **Database:** SQLite via GRDB.swift or SQLite.swift
- **Markdown:** MarkdownUI (SwiftUI-native)
- **Networking:** URLSession (built-in) or Alamofire
- **PostgreSQL (if direct):** PostgresClientKit

### API Server (if Option B)
- **Language:** Go (reuses vimango code patterns)
- **Framework:** Gin or Echo (lightweight)
- **Auth:** API key in header or JWT
- **Deployment:** Same server as PostgreSQL, Docker, or cloud function

---

## Development Phases

### Phase 1: Minimum Viable Product (MVP)
- [ ] iOS app with SwiftUI
- [ ] Direct PostgreSQL connection (Option A)
- [ ] List view of notes (title, context, folder, modified date)
- [ ] Detail view with markdown rendering
- [ ] Basic filtering by context/folder
- [ ] Pull-to-refresh sync
- **No images in MVP** - show placeholder or link

### Phase 2: Offline Support
- [ ] Local SQLite cache
- [ ] Incremental sync on app launch
- [ ] Offline banner/indicator
- [ ] Full-text search of cached notes (optional)

### Phase 3: Image Support
- [ ] Either: Google Sign-In + Drive API
- [ ] Or: Build API server with image proxy endpoint
- [ ] Image caching in app
- [ ] Lazy loading in markdown views

### Phase 4: Polish
- [ ] Star/archive filters
- [ ] Keyword filtering
- [ ] Search across notes
- [ ] Dark mode / theme support
- [ ] iPad layout optimization
- [ ] Widget for recent/starred notes

---

## Security Considerations

### Storing PostgreSQL Credentials (Option A)
- Use iOS Keychain for password storage
- Consider server-side SSL/TLS requirement
- May want VPN requirement for database access

### API Key (Option B)
- Generate unique API key per device
- Store in Keychain
- Implement rate limiting server-side
- Consider token expiration/refresh

### Data at Rest
- iOS provides hardware encryption by default
- Consider additional encryption for sensitive notes
- SQLite encryption available via SQLCipher if needed

---

## Estimated Complexity

| Component | Effort | Notes |
|-----------|--------|-------|
| Basic SwiftUI list/detail | Low | Standard iOS patterns |
| PostgreSQL direct connection | Low | Library handles complexity |
| SQLite local cache | Medium | Schema, CRUD, migrations |
| Sync logic | Medium | Timestamp tracking, conflict handling |
| Markdown rendering | Low | MarkdownUI handles this |
| Google Drive images | High | OAuth flow, API integration |
| API server (Go) | Medium | Can reuse vimango patterns |
| Image proxy endpoint | Low | Simple passthrough |

**Overall Assessment:** This is a achievable project. The MVP (Phase 1) could be built in a focused sprint. The complexity mainly scales with image handling requirements.

---

## Open Questions

1. **Image priority:** How important is image support for initial release?
2. **Sync frequency:** On-demand only, or periodic background refresh?
3. **Search:** Local search of cached content, or server-side FTS?
4. **Authentication:** Single user (your account), or multi-user support?
5. **Distribution:** Personal use (TestFlight), or App Store?
6. **iPad/Mac:** Universal app, or iPhone-only initially?

---

## Appendix: Relevant Vimango Source Files

| File | Purpose |
|------|---------|
| `init.go` | SQLite schema, default UUIDs |
| `synchronize.go` | Sync logic, PostgreSQL queries |
| `dbfunc.go` | Database CRUD operations |
| `common.go` | Data structures, config schema |
| `auth/drive.go` | Google Drive OAuth patterns |
| `image_cache.go` | Image caching implementation |
| `term_misc.go` | Image URL parsing, regex patterns |

---

*Document generated: 2026-01-29*
*For: vimango iOS companion app planning*
