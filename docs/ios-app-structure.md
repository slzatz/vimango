# Vimango iOS App - Minimal Code Structure

This document sketches out what a minimal read-only vimango iOS app would look like. This is the simplest viable implementation - direct PostgreSQL connection, no local caching, no images.

## Project Structure

```
VimaNotes/
├── VimaNotes.xcodeproj          # Xcode project file
├── VimaNotes/
│   ├── VimaNotesApp.swift       # App entry point
│   ├── ContentView.swift        # Main container view
│   ├── Models/
│   │   ├── Note.swift           # Note data model
│   │   └── Container.swift      # Context/Folder model
│   ├── Views/
│   │   ├── NoteListView.swift   # List of notes
│   │   ├── NoteRowView.swift    # Single row in list
│   │   ├── NoteDetailView.swift # Full note with markdown
│   │   └── FilterView.swift     # Context/folder picker
│   ├── Services/
│   │   └── DatabaseService.swift # PostgreSQL connection
│   ├── Config.swift             # Server credentials
│   └── Assets.xcassets/         # App icons, colors
└── Package.swift                # Swift Package dependencies
```

## Dependencies

```swift
// Package.swift (Swift Package Manager)
dependencies: [
    // PostgreSQL client
    .package(url: "https://github.com/codewinsdotcom/PostgresClientKit", from: "1.5.0"),
    // Markdown rendering
    .package(url: "https://github.com/gonzalezreal/MarkdownUI", from: "2.0.0"),
]
```

---

## Core Files

### VimaNotesApp.swift
The app entry point - minimal boilerplate.

```swift
import SwiftUI

@main
struct VimaNotesApp: App {
    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}
```

### ContentView.swift
Main container - handles navigation structure.

```swift
import SwiftUI

struct ContentView: View {
    @StateObject private var viewModel = NotesViewModel()

    var body: some View {
        NavigationStack {
            NoteListView(viewModel: viewModel)
        }
    }
}
```

---

## Models

### Note.swift
Maps to vimango's `task` table.

```swift
import Foundation

struct Note: Identifiable {
    let tid: Int
    let title: String
    let note: String?          // Markdown content
    let star: Bool
    let archived: Bool
    let contextTitle: String
    let folderTitle: String
    let modified: Date

    var id: Int { tid }

    // Preview/placeholder for development
    static let example = Note(
        tid: 1,
        title: "Sample Note",
        note: "# Hello\n\nThis is a **sample** note with markdown.",
        star: true,
        archived: false,
        contextTitle: "work",
        folderTitle: "projects",
        modified: Date()
    )
}
```

### Container.swift
For contexts and folders.

```swift
import Foundation

struct Container: Identifiable, Hashable {
    let tid: Int
    let uuid: String
    let title: String
    let star: Bool

    var id: Int { tid }

    static let all = Container(tid: 0, uuid: "", title: "All", star: false)
}
```

---

## Views

### NoteListView.swift
The main list of notes.

```swift
import SwiftUI

struct NoteListView: View {
    @ObservedObject var viewModel: NotesViewModel
    @State private var searchText = ""
    @State private var selectedContext: Container = .all

    var filteredNotes: [Note] {
        var notes = viewModel.notes

        // Filter by context
        if selectedContext.tid != 0 {
            notes = notes.filter { $0.contextTitle == selectedContext.title }
        }

        // Filter by search
        if !searchText.isEmpty {
            notes = notes.filter {
                $0.title.localizedCaseInsensitiveContains(searchText)
            }
        }

        return notes
    }

    var body: some View {
        List(filteredNotes) { note in
            NavigationLink(destination: NoteDetailView(note: note)) {
                NoteRowView(note: note)
            }
        }
        .navigationTitle("Notes")
        .searchable(text: $searchText, prompt: "Search notes")
        .refreshable {
            await viewModel.refresh()
        }
        .toolbar {
            ToolbarItem(placement: .navigationBarTrailing) {
                Menu {
                    Picker("Context", selection: $selectedContext) {
                        Text("All").tag(Container.all)
                        ForEach(viewModel.contexts) { context in
                            Text(context.title).tag(context)
                        }
                    }
                } label: {
                    Label("Filter", systemImage: "line.3.horizontal.decrease.circle")
                }
            }
        }
        .task {
            // Load data when view appears
            await viewModel.loadNotes()
            await viewModel.loadContainers()
        }
    }
}
```

### NoteRowView.swift
A single row in the notes list.

```swift
import SwiftUI

struct NoteRowView: View {
    let note: Note

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(note.title)
                    .font(.headline)
                    .lineLimit(1)

                if note.star {
                    Image(systemName: "star.fill")
                        .foregroundColor(.yellow)
                        .font(.caption)
                }
            }

            HStack {
                Text(note.contextTitle)
                    .font(.caption)
                    .foregroundColor(.secondary)
                    .padding(.horizontal, 6)
                    .padding(.vertical, 2)
                    .background(Color.blue.opacity(0.1))
                    .cornerRadius(4)

                Text(note.folderTitle)
                    .font(.caption)
                    .foregroundColor(.secondary)
                    .padding(.horizontal, 6)
                    .padding(.vertical, 2)
                    .background(Color.green.opacity(0.1))
                    .cornerRadius(4)

                Spacer()

                Text(note.modified, style: .date)
                    .font(.caption2)
                    .foregroundColor(.secondary)
            }
        }
        .padding(.vertical, 4)
    }
}
```

### NoteDetailView.swift
Full note display with markdown rendering.

```swift
import SwiftUI
import MarkdownUI

struct NoteDetailView: View {
    let note: Note

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                // Header
                VStack(alignment: .leading, spacing: 8) {
                    Text(note.title)
                        .font(.largeTitle)
                        .fontWeight(.bold)

                    HStack {
                        Label(note.contextTitle, systemImage: "folder")
                        Label(note.folderTitle, systemImage: "doc")
                        if note.star {
                            Label("Starred", systemImage: "star.fill")
                                .foregroundColor(.yellow)
                        }
                    }
                    .font(.caption)
                    .foregroundColor(.secondary)
                }

                Divider()

                // Markdown content
                if let content = note.note, !content.isEmpty {
                    Markdown(content)
                        .markdownTheme(.gitHub)  // or custom theme
                } else {
                    Text("No content")
                        .foregroundColor(.secondary)
                        .italic()
                }
            }
            .padding()
        }
        .navigationBarTitleDisplayMode(.inline)
    }
}
```

---

## View Model

### NotesViewModel.swift
Manages data loading and state.

```swift
import Foundation

@MainActor
class NotesViewModel: ObservableObject {
    @Published var notes: [Note] = []
    @Published var contexts: [Container] = []
    @Published var folders: [Container] = []
    @Published var isLoading = false
    @Published var errorMessage: String?

    private let db = DatabaseService.shared

    func loadNotes() async {
        isLoading = true
        errorMessage = nil

        do {
            notes = try await db.fetchNotes()
        } catch {
            errorMessage = "Failed to load notes: \(error.localizedDescription)"
        }

        isLoading = false
    }

    func loadContainers() async {
        do {
            contexts = try await db.fetchContexts()
            folders = try await db.fetchFolders()
        } catch {
            // Non-fatal, just log
            print("Failed to load containers: \(error)")
        }
    }

    func refresh() async {
        await loadNotes()
        await loadContainers()
    }
}
```

---

## Database Service

### DatabaseService.swift
PostgreSQL connection using PostgresClientKit.

```swift
import Foundation
import PostgresClientKit

actor DatabaseService {
    static let shared = DatabaseService()

    private var connection: Connection?

    private func getConnection() throws -> Connection {
        if let conn = connection, !conn.isClosed {
            return conn
        }

        // Create new connection
        var config = ConnectionConfiguration()
        config.host = Config.pgHost
        config.port = Config.pgPort
        config.database = Config.pgDatabase
        config.user = Config.pgUser
        config.credential = .scramSHA256(password: Config.pgPassword)
        config.ssl = true  // Require SSL for security

        connection = try Connection(configuration: config)
        return connection!
    }

    func fetchNotes() throws -> [Note] {
        let conn = try getConnection()

        let sql = """
            SELECT t.tid, t.title, t.note, t.star, t.archived,
                   t.modified, c.title AS context_title, f.title AS folder_title
            FROM task t
            JOIN context c ON c.uuid = t.context_uuid
            JOIN folder f ON f.uuid = t.folder_uuid
            WHERE t.deleted = false AND t.archived = false
            ORDER BY t.modified DESC
            LIMIT 500
            """

        let statement = try conn.prepareStatement(text: sql)
        defer { statement.close() }

        let cursor = try statement.execute()
        defer { cursor.close() }

        var notes: [Note] = []

        for row in cursor {
            let columns = try row.get().columns

            let note = Note(
                tid: try columns[0].int(),
                title: try columns[1].string(),
                note: try columns[2].optionalString(),
                star: try columns[3].bool(),
                archived: try columns[4].bool(),
                contextTitle: try columns[6].string(),
                folderTitle: try columns[7].string(),
                modified: try columns[5].timestampWithTimeZone().date
            )
            notes.append(note)
        }

        return notes
    }

    func fetchContexts() throws -> [Container] {
        let conn = try getConnection()

        let sql = """
            SELECT tid, uuid, title, star FROM context
            WHERE deleted = false ORDER BY title
            """

        let statement = try conn.prepareStatement(text: sql)
        defer { statement.close() }

        let cursor = try statement.execute()
        defer { cursor.close() }

        var containers: [Container] = []

        for row in cursor {
            let columns = try row.get().columns
            let container = Container(
                tid: try columns[0].int(),
                uuid: try columns[1].string(),
                title: try columns[2].string(),
                star: try columns[3].bool()
            )
            containers.append(container)
        }

        return containers
    }

    func fetchFolders() throws -> [Container] {
        let conn = try getConnection()

        let sql = """
            SELECT tid, uuid, title, star FROM folder
            WHERE deleted = false ORDER BY title
            """

        let statement = try conn.prepareStatement(text: sql)
        defer { statement.close() }

        let cursor = try statement.execute()
        defer { cursor.close() }

        var containers: [Container] = []

        for row in cursor {
            let columns = try row.get().columns
            let container = Container(
                tid: try columns[0].int(),
                uuid: try columns[1].string(),
                title: try columns[2].string(),
                star: try columns[3].bool()
            )
            containers.append(container)
        }

        return containers
    }
}
```

### Config.swift
Store connection details (in real app, use Keychain).

```swift
import Foundation

enum Config {
    // In production, store these in Keychain, not in code!
    static let pgHost = "your-server.com"
    static let pgPort = 5432
    static let pgDatabase = "listmanager"
    static let pgUser = "your_user"
    static let pgPassword = "your_password"  // Move to Keychain!
}
```

---

## What This Gets You

With the code above, you'd have:

1. **Note list** with pull-to-refresh
2. **Search** by title
3. **Filter** by context
4. **Note detail** with rendered markdown
5. **Star indicators**
6. **Context/folder badges**

## What's NOT Included (Future Phases)

- Local SQLite caching (offline support)
- Images (Google Drive integration)
- Keywords display/filtering
- Full-text search
- Folder filtering (easy to add)
- Settings screen
- Error handling UI (loading states, retry)

---

## Learning Resources

Before building this, I'd recommend:

1. **Apple's SwiftUI Tutorial** - "Introducing SwiftUI"
   https://developer.apple.com/tutorials/swiftui

2. **100 Days of SwiftUI** (free course by Paul Hudson)
   https://www.hackingwithswift.com/100/swiftui

3. **MarkdownUI Documentation**
   https://github.com/gonzalezreal/MarkdownUI

The app structure above follows standard SwiftUI patterns you'll learn in these tutorials.

---

## Network Requirements

Your PostgreSQL server needs to be reachable from your iPhone:

**Option 1: Same local network**
- Phone on same WiFi as server
- Use local IP (e.g., 192.168.1.x)

**Option 2: VPN**
- Connect phone to home VPN
- Access server via VPN IP

**Option 3: Internet exposed**
- Port forward PostgreSQL (risky without SSL)
- Or use a tunnel service (ngrok, tailscale)

**Recommended: Tailscale**
- Free for personal use
- Creates secure mesh network
- Install on server + phone
- Access via Tailscale IP, encrypted automatically
