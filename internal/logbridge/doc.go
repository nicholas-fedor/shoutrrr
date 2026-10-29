// Package logbridge connects the loggers the library is given to log/slog.
//
// [New] turns the logger a service was given into the slog logger that
// receives its leveled records. A [types.SlogLogger] carries its own slog
// logger. Any other logger receives each record as one line of text in the
// format of slog's default handler, such as "WARN sending without TLS". [With]
// adds attributes to every record, as the router does with the service's ID.
//
// [Log] logs a record with a given source, and [CallerPC] finds that source
// further up the stack. Records then name the service's call, not the
// library's logging code, as their source.
package logbridge
