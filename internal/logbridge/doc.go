// Package logbridge connects the loggers the library is given to log/slog.
//
// [New] turns the logger a service was given into the slog logger that
// receives its leveled records. A [types.SlogLogger] carries its own slog
// logger. Any other logger receives each record as one line of text in the
// format of slog's default handler, such as "WARN sending without TLS". [With]
// adds attributes to every record, as the router does with the service's ID.
//
// [Log] and [LogAttrs] log a record whose source is a call further up the
// stack. Records then name the service's call, not the library's logging
// code, as their source. LogAttrs takes [slog.Attr] values, so a record
// that the logger drops costs no allocation.
package logbridge
