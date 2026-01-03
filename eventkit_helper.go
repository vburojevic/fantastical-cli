//go:build darwin
// +build darwin

package main

const eventKitHelperSource = `import Foundation
import EventKit

struct Options {
    var format: String = "plain"
    var noInput: Bool = false
    var calendars: [String] = []
    var from: String? = nil
    var to: String? = nil
    var limit: Int? = nil
    var includeAllDay: Bool = true
    var includeDeclined: Bool = false
}

func eprintln(_ message: String) {
    if let data = (message + "\n").data(using: .utf8) {
        FileHandle.standardError.write(data)
    }
}

func usage() {
    let text = """
USAGE:
  eventkit calendars [--json|--plain] [--no-input]
  eventkit events [--from <date>] [--to <date>] [--calendar <name>] [--limit N]
                  [--include-all-day] [--include-declined]
                  [--json|--plain] [--no-input]

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
        case "--json":
            opts.format = "json"
        case "--plain":
            opts.format = "plain"
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

func ensureAuthorized(store: EKEventStore, noInput: Bool) -> Bool {
    let status = EKEventStore.authorizationStatus(for: .event)
    switch status {
    case .authorized:
        return true
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
    case .other: return "other"
    @unknown default: return "unknown"
    }
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

    for cal in calendars {
        print("\(cal.title)\t(\(cal.source.title))")
    }
}

func outputEvents(_ events: [EKEvent], format: String) {
    if format == "json" {
        let items = events.map {
            EventOutput(id: $0.eventIdentifier, title: $0.title ?? "", calendar: $0.calendar.title, calendarId: $0.calendar.calendarIdentifier, start: $0.startDate, end: $0.endDate, allDay: $0.isAllDay, location: $0.location, notes: $0.notes)
        }
        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .iso8601
        if let data = try? encoder.encode(items), let text = String(data: data, encoding: .utf8) {
            print(text)
        }
        return
    }

    let formatter = DateFormatter()
    formatter.locale = Locale(identifier: "en_US_POSIX")
    formatter.timeZone = TimeZone.current
    formatter.dateFormat = "yyyy-MM-dd HH:mm"

    for event in events {
        let start = formatter.string(from: event.startDate)
        let end = formatter.string(from: event.endDate)
        let title = event.title ?? ""
        print("\(start)\t\(end)\t\(event.calendar.title)\t\(title)")
    }
}

let args = Array(CommandLine.arguments.dropFirst())
if let (command, opts) = parseArgs(args) {
    let store = EKEventStore()
    guard ensureAuthorized(store: store, noInput: opts.noInput) else {
        exit(1)
    }

    switch command {
    case "calendars":
        let calendars = store.calendars(for: .event).sorted { $0.title.lowercased() < $1.title.lowercased() }
        outputCalendars(calendars, format: opts.format)
    case "events":
        let now = Date()
        var fromDate: Date = startOfDay(now)
        var toDate: Date = endOfDay(now)

        if let fromValue = opts.from {
            if let (parsed, dateOnly) = parseDate(fromValue) {
                fromDate = dateOnly ? startOfDay(parsed) : parsed
            } else {
                eprintln("invalid --from value: \(fromValue)")
                exit(2)
            }
        }

        if let toValue = opts.to {
            if let (parsed, dateOnly) = parseDate(toValue) {
                toDate = dateOnly ? endOfDay(parsed) : parsed
            } else {
                eprintln("invalid --to value: \(toValue)")
                exit(2)
            }
        }

        if toDate < fromDate {
            eprintln("--to must be after --from")
            exit(2)
        }

        let calendars: [EKCalendar]
        if opts.calendars.isEmpty {
            calendars = store.calendars(for: .event)
        } else {
            let all = store.calendars(for: .event)
            calendars = all.filter { cal in
                opts.calendars.contains(cal.title)
            }
        }

        let predicate = store.predicateForEvents(withStart: fromDate, end: toDate, calendars: calendars)
        var events = store.events(matching: predicate)

        if !opts.includeAllDay {
            events = events.filter { !$0.isAllDay }
        }
        if !opts.includeDeclined {
            events = events.filter { $0.participationStatus != .declined }
        }

        events.sort { $0.startDate < $1.startDate }

        if let limit = opts.limit, limit > 0, events.count > limit {
            events = Array(events.prefix(limit))
        }

        outputEvents(events, format: opts.format)
    default:
        eprintln("unknown subcommand: \(command)")
        usage()
        exit(2)
    }
}

`
