//go:build darwin
// +build darwin

package main

const eventKitHelperSource = `import Foundation
import EventKit

struct Options {
    var format: String = "plain"
    var noInput: Bool = false
    var calendars: [String] = []
    var calendarIds: [String] = []
    var from: String? = nil
    var to: String? = nil
    var days: Int? = nil
    var today: Bool = false
    var tomorrow: Bool = false
    var thisWeek: Bool = false
    var nextWeek: Bool = false
    var limit: Int? = nil
    var includeAllDay: Bool = true
    var includeDeclined: Bool = false
    var sort: String = "start"
    var timezone: String? = nil
    var query: String? = nil
}

func eprintln(_ message: String) {
    if let data = (message + "\n").data(using: .utf8) {
        FileHandle.standardError.write(data)
    }
}

func usage() {
    let text = """
USAGE:
  eventkit status [--format plain|json]
  eventkit calendars [--format plain|json|table] [--no-input]
  eventkit events [--from <date>] [--to <date>] [--days N] [--today|--tomorrow|--this-week|--next-week]
                 [--calendar <name>] [--calendar-id <id>]
                 [--query <text>] [--sort start|end|title|calendar]
                 [--tz <iana>] [--limit N]
                 [--include-all-day] [--include-declined]
                 [--format plain|json|table] [--no-input]

DATE FORMATS:
  YYYY-MM-DD
  YYYY-MM-DDTHH:MM
  YYYY-MM-DDTHH:MM:SS
"""
    print(text)
}

func parseArgs(_ args: [String]) -> (String, Options)? {
    if args.isEmpty {
        usage()
        return nil
    }

    let command = args[0]
    var opts = Options()
    var i = 1
    while i < args.count {
        let arg = args[i]
        switch arg {
        case "--format":
            i += 1
            if i >= args.count {
                eprintln("missing value for --format")
                usage()
                return nil
            }
            opts.format = args[i]
        case "--json":
            opts.format = "json"
        case "--plain":
            opts.format = "plain"
        case "--table":
            opts.format = "table"
        case "--no-input":
            opts.noInput = true
        case "--calendar":
            i += 1
            if i >= args.count {
                eprintln("missing value for --calendar")
                usage()
                return nil
            }
            opts.calendars.append(args[i])
        case "--calendar-id":
            i += 1
            if i >= args.count {
                eprintln("missing value for --calendar-id")
                usage()
                return nil
            }
            opts.calendarIds.append(args[i])
        case "--from":
            i += 1
            if i >= args.count {
                eprintln("missing value for --from")
                usage()
                return nil
            }
            opts.from = args[i]
        case "--to":
            i += 1
            if i >= args.count {
                eprintln("missing value for --to")
                usage()
                return nil
            }
            opts.to = args[i]
        case "--days":
            i += 1
            if i >= args.count {
                eprintln("missing value for --days")
                usage()
                return nil
            }
            if let value = Int(args[i]) {
                opts.days = value
            } else {
                eprintln("invalid --days value: \(args[i])")
                return nil
            }
        case "--today":
            opts.today = true
        case "--tomorrow":
            opts.tomorrow = true
        case "--this-week":
            opts.thisWeek = true
        case "--next-week":
            opts.nextWeek = true
        case "--limit":
            i += 1
            if i >= args.count {
                eprintln("missing value for --limit")
                usage()
                return nil
            }
            if let value = Int(args[i]) {
                opts.limit = value
            } else {
                eprintln("invalid --limit value: \(args[i])")
                return nil
            }
        case "--include-all-day":
            opts.includeAllDay = true
        case "--no-all-day":
            opts.includeAllDay = false
        case "--include-declined":
            opts.includeDeclined = true
        case "--sort":
            i += 1
            if i >= args.count {
                eprintln("missing value for --sort")
                usage()
                return nil
            }
            opts.sort = args[i]
        case "--tz":
            i += 1
            if i >= args.count {
                eprintln("missing value for --tz")
                usage()
                return nil
            }
            opts.timezone = args[i]
        case "--query":
            i += 1
            if i >= args.count {
                eprintln("missing value for --query")
                usage()
                return nil
            }
            opts.query = args[i]
        case "--help", "-h":
            usage()
            return nil
        default:
            if arg.hasPrefix("--") {
                eprintln("unknown flag: \(arg)")
                usage()
                return nil
            } else {
                eprintln("unexpected argument: \(arg)")
                usage()
                return nil
            }
        }
        i += 1
    }

    return (command, opts)
}

func parseDate(_ value: String) -> (Date, Bool)? {
    let formats = ["yyyy-MM-dd'T'HH:mm:ss", "yyyy-MM-dd'T'HH:mm", "yyyy-MM-dd"]
    let formatter = DateFormatter()
    formatter.locale = Locale(identifier: "en_US_POSIX")
    formatter.timeZone = TimeZone.current

    for format in formats {
        formatter.dateFormat = format
        if let date = formatter.date(from: value) {
            return (date, format == "yyyy-MM-dd")
        }
    }
    return nil
}

func startOfDay(_ date: Date) -> Date {
    return Calendar.current.startOfDay(for: date)
}

func endOfDay(_ date: Date) -> Date {
    let start = Calendar.current.startOfDay(for: date)
    guard let nextDay = Calendar.current.date(byAdding: .day, value: 1, to: start) else {
        return date
    }
    return nextDay.addingTimeInterval(-1)
}

func resolveTimeZone(_ value: String?) -> TimeZone? {
    if let value = value, !value.isEmpty {
        return TimeZone(identifier: value)
    }
    return TimeZone.current
}

func resolveDateRange(_ opts: Options) -> (Date, Date)? {
    let now = Date()
    let calendar = Calendar.current

    let presetCount = [opts.today, opts.tomorrow, opts.thisWeek, opts.nextWeek].filter { $0 }.count
    if presetCount > 1 {
        eprintln("only one of --today/--tomorrow/--this-week/--next-week can be used")
        return nil
    }
    if presetCount > 0 && (opts.from != nil || opts.to != nil || opts.days != nil) {
        eprintln("--from/--to/--days cannot be combined with date shortcuts")
        return nil
    }
    if opts.days != nil && (opts.from != nil || opts.to != nil) {
        eprintln("--days cannot be combined with --from/--to")
        return nil
    }

    if let days = opts.days {
        if days <= 0 {
            eprintln("--days must be greater than 0")
            return nil
        }
        guard let toDate = calendar.date(byAdding: .day, value: days, to: now) else {
            return nil
        }
        return (now, toDate)
    }

    if opts.today {
        return (startOfDay(now), endOfDay(now))
    }
    if opts.tomorrow {
        let tomorrow = calendar.date(byAdding: .day, value: 1, to: now) ?? now
        return (startOfDay(tomorrow), endOfDay(tomorrow))
    }
    if opts.thisWeek {
        if let interval = calendar.dateInterval(of: .weekOfYear, for: now) {
            return (interval.start, interval.end.addingTimeInterval(-1))
        }
        return nil
    }
    if opts.nextWeek {
        let next = calendar.date(byAdding: .day, value: 7, to: now) ?? now
        if let interval = calendar.dateInterval(of: .weekOfYear, for: next) {
            return (interval.start, interval.end.addingTimeInterval(-1))
        }
        return nil
    }

    var fromDate = startOfDay(now)
    var toDate = endOfDay(now)
    var fromDateOnly = false
    var toDateOnly = false

    if let fromValue = opts.from {
        if let (parsed, dateOnly) = parseDate(fromValue) {
            fromDateOnly = dateOnly
            fromDate = dateOnly ? startOfDay(parsed) : parsed
        } else {
            eprintln("invalid --from value: \(fromValue)")
            return nil
        }
    }

    if let toValue = opts.to {
        if let (parsed, dateOnly) = parseDate(toValue) {
            toDateOnly = dateOnly
            toDate = dateOnly ? endOfDay(parsed) : parsed
        } else {
            eprintln("invalid --to value: \(toValue)")
            return nil
        }
    }

    if opts.from != nil && opts.to == nil {
        if fromDateOnly {
            toDate = endOfDay(fromDate)
        } else if let plus = calendar.date(byAdding: .day, value: 1, to: fromDate) {
            toDate = plus
        }
    }

    if opts.to != nil && opts.from == nil {
        if toDateOnly {
            fromDate = startOfDay(toDate)
        } else if let minus = calendar.date(byAdding: .day, value: -1, to: toDate) {
            fromDate = minus
        }
    }

    if toDate < fromDate {
        eprintln("--to must be after --from")
        return nil
    }

    return (fromDate, toDate)
}

func ensureAuthorized(store: EKEventStore, noInput: Bool) -> Bool {
    let status = EKEventStore.authorizationStatus(for: .event)
    switch status {
    case .authorized:
        return true
    case .fullAccess:
        return true
    case .writeOnly:
        eprintln("Calendar access is write-only; cannot list events.")
        return false
    case .notDetermined:
        if noInput {
            eprintln("Calendar access not granted. Re-run without --no-input to trigger the permission prompt.")
            return false
        }
        let semaphore = DispatchSemaphore(value: 0)
        var granted = false
        store.requestAccess(to: .event) { ok, _ in
            granted = ok
            semaphore.signal()
        }
        _ = semaphore.wait(timeout: .now() + 30)
        if !granted {
            eprintln("Calendar access denied.")
        }
        return granted
    case .denied:
        eprintln("Calendar access denied. Enable access in System Settings > Privacy & Security > Calendars.")
        return false
    case .restricted:
        eprintln("Calendar access restricted by system policy.")
        return false
    @unknown default:
        eprintln("Calendar access unavailable.")
        return false
    }
}

func authorizationStatusString(_ status: EKAuthorizationStatus) -> String {
    switch status {
    case .authorized:
        return "authorized"
    case .fullAccess:
        return "full_access"
    case .writeOnly:
        return "write_only"
    case .notDetermined:
        return "not_determined"
    case .denied:
        return "denied"
    case .restricted:
        return "restricted"
    @unknown default:
        return "unknown"
    }
}

struct CalendarOutput: Codable {
    let id: String
    let title: String
    let source: String
    let type: String
    let allowsModifications: Bool
}

struct EventOutput: Codable {
    let id: String
    let title: String
    let calendar: String
    let calendarId: String
    let start: Date
    let end: Date
    let allDay: Bool
    let location: String?
    let notes: String?
}

func calendarTypeName(_ type: EKCalendarType) -> String {
    switch type {
    case .local: return "local"
    case .calDAV: return "caldav"
    case .exchange: return "exchange"
    case .subscription: return "subscription"
    case .birthday: return "birthday"
    @unknown default: return "unknown"
    }
}

func isDeclinedByCurrentUser(_ event: EKEvent) -> Bool {
    guard let attendees = event.attendees else {
        return false
    }
    for attendee in attendees where attendee.isCurrentUser {
        if attendee.participantStatus == .declined {
            return true
        }
    }
    return false
}

func renderTable(headers: [String], rows: [[String]]) -> String {
    var widths = headers.map { $0.count }
    for row in rows {
        for (idx, value) in row.enumerated() {
            if value.count > widths[idx] {
                widths[idx] = value.count
            }
        }
    }

    func pad(_ value: String, _ width: Int) -> String {
        if value.count >= width {
            return value
        }
        return value + String(repeating: " ", count: width - value.count)
    }

    var lines: [String] = []
    let headerLine = headers.enumerated().map { pad($0.element, widths[$0.offset]) }.joined(separator: "  ")
    lines.append(headerLine)
    let separator = widths.map { String(repeating: "-", count: $0) }.joined(separator: "  ")
    lines.append(separator)

    for row in rows {
        let line = row.enumerated().map { pad($0.element, widths[$0.offset]) }.joined(separator: "  ")
        lines.append(line)
    }
    return lines.joined(separator: "\n")
}

func outputStatus(_ status: EKAuthorizationStatus, format: String) {
    let statusString = authorizationStatusString(status)
    if format == "json" {
        let payload: [String: Any] = [
            "status": statusString,
            "canPrompt": status == .notDetermined
        ]
        if let data = try? JSONSerialization.data(withJSONObject: payload, options: []),
           let text = String(data: data, encoding: .utf8) {
            print(text)
        }
        return
    }
    print(statusString)
}

func outputCalendars(_ calendars: [EKCalendar], format: String) {
    if format == "json" {
        let items = calendars.map {
            CalendarOutput(id: $0.calendarIdentifier, title: $0.title, source: $0.source.title, type: calendarTypeName($0.type), allowsModifications: $0.allowsContentModifications)
        }
        let encoder = JSONEncoder()
        if let data = try? encoder.encode(items), let text = String(data: data, encoding: .utf8) {
            print(text)
        }
        return
    }

    if format == "table" {
        let rows = calendars.map { [$0.title, $0.source.title, calendarTypeName($0.type), $0.calendarIdentifier] }
        let table = renderTable(headers: ["Title", "Source", "Type", "ID"], rows: rows)
        print(table)
        return
    }

    for cal in calendars {
        print("\(cal.title)\t(\(cal.source.title))")
    }
}

func outputEvents(_ events: [EKEvent], format: String, timeZone: TimeZone) {
    if format == "json" {
        let items = events.map {
            EventOutput(id: $0.eventIdentifier, title: $0.title ?? "", calendar: $0.calendar.title, calendarId: $0.calendar.calendarIdentifier, start: $0.startDate, end: $0.endDate, allDay: $0.isAllDay, location: $0.location, notes: $0.notes)
        }
        let encoder = JSONEncoder()
        let formatter = ISO8601DateFormatter()
        formatter.timeZone = timeZone
        formatter.formatOptions = [.withInternetDateTime]
        encoder.dateEncodingStrategy = .custom { date, encoder in
            var container = encoder.singleValueContainer()
            let text = formatter.string(from: date)
            try container.encode(text)
        }
        if let data = try? encoder.encode(items), let text = String(data: data, encoding: .utf8) {
            print(text)
        }
        return
    }

    let formatter = DateFormatter()
    formatter.locale = Locale(identifier: "en_US_POSIX")
    formatter.timeZone = timeZone
    formatter.dateFormat = "yyyy-MM-dd HH:mm"

    if format == "table" {
        let rows = events.map { event in
            let start = formatter.string(from: event.startDate)
            let end = formatter.string(from: event.endDate)
            let title = event.title ?? ""
            return [start, end, event.calendar.title, title]
        }
        let table = renderTable(headers: ["Start", "End", "Calendar", "Title"], rows: rows)
        print(table)
        return
    }

    for event in events {
        let start = formatter.string(from: event.startDate)
        let end = formatter.string(from: event.endDate)
        let title = event.title ?? ""
        print("\(start)\t\(end)\t\(event.calendar.title)\t\(title)")
    }
}

let args = Array(CommandLine.arguments.dropFirst())
if let (command, opts) = parseArgs(args) {
    let format = opts.format.lowercased()
    let allowedFormats = ["plain", "json", "table"]
    if !allowedFormats.contains(format) {
        eprintln("invalid --format value: \(format)")
        exit(2)
    }
    if command == "status" && format == "table" {
        eprintln("status does not support table output")
        exit(2)
    }
    if command == "status" {
        outputStatus(EKEventStore.authorizationStatus(for: .event), format: format)
        exit(0)
    }

    let store = EKEventStore()
    guard ensureAuthorized(store: store, noInput: opts.noInput) else {
        exit(1)
    }

    let outputTimeZone = resolveTimeZone(opts.timezone)
    if outputTimeZone == nil {
        eprintln("invalid --tz value: \(opts.timezone ?? "")")
        exit(2)
    }

    switch command {
    case "calendars":
        let calendars = store.calendars(for: .event).sorted { $0.title.lowercased() < $1.title.lowercased() }
        outputCalendars(calendars, format: format)
    case "events":
        guard let (fromDate, toDate) = resolveDateRange(opts) else {
            exit(2)
        }

        let calendars: [EKCalendar]
        if opts.calendars.isEmpty && opts.calendarIds.isEmpty {
            calendars = store.calendars(for: .event)
        } else {
            let all = store.calendars(for: .event)
            calendars = all.filter { cal in
                opts.calendars.contains(cal.title) || opts.calendarIds.contains(cal.calendarIdentifier)
            }
        }

        let predicate = store.predicateForEvents(withStart: fromDate, end: toDate, calendars: calendars)
        var events = store.events(matching: predicate)

        if let query = opts.query?.lowercased(), !query.isEmpty {
            events = events.filter { event in
                let title = (event.title ?? "").lowercased()
                let location = (event.location ?? "").lowercased()
                let notes = (event.notes ?? "").lowercased()
                return title.contains(query) || location.contains(query) || notes.contains(query)
            }
        }

        if !opts.includeAllDay {
            events = events.filter { !$0.isAllDay }
        }
        if !opts.includeDeclined {
            events = events.filter { !isDeclinedByCurrentUser($0) }
        }

        switch opts.sort.lowercased() {
        case "start":
            events.sort { $0.startDate < $1.startDate }
        case "end":
            events.sort { $0.endDate < $1.endDate }
        case "title":
            events.sort { ($0.title ?? "").lowercased() < ($1.title ?? "").lowercased() }
        case "calendar":
            events.sort {
                let lhs = $0.calendar.title.lowercased()
                let rhs = $1.calendar.title.lowercased()
                if lhs == rhs {
                    return $0.startDate < $1.startDate
                }
                return lhs < rhs
            }
        default:
            events.sort { $0.startDate < $1.startDate }
        }

        if let limit = opts.limit, limit > 0, events.count > limit {
            events = Array(events.prefix(limit))
        }

        outputEvents(events, format: format, timeZone: outputTimeZone ?? TimeZone.current)
    default:
        eprintln("unknown subcommand: \(command)")
        usage()
        exit(2)
    }
}
`
