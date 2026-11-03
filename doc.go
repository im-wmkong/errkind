// Package errkind provides business error identities, instances and cause chains.
//
// Define registers a stable Kind in the default Registry. NewRegistry isolates
// definitions when applications or tests need independent error catalogs.
// A Kind is an error and an errors.Is target. Kind.New and Kind.Wrap create
// *Error instances with literal messages and attributes constructed by With.
//
// Use errors.Is to match a Kind and errors.As to read an *Error's Kind, Message
// and Attrs. Standard wrappers and joined errors follow Go's traversal rules.
// Applications choose logging fields, protocol mappings and handling policies.
package errkind
