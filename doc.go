// Package errkind provides business error identities, instances and cause chains.
//
// Define registers a stable Kind in the default Registry. NewRegistry isolates
// definitions when applications or tests need independent error catalogs.
// Kind and Error are interfaces backed by private implementations. Registered
// identities are immutable. A Kind is an error and an errors.Is target.
// Kind.New and Kind.Wrap return Error instances with literal messages and
// attributes constructed by With. Wrap with a nil cause returns nil.
//
// Use errors.Is to match a Kind and errors.As with a *Kind target to read the
// first business identity from either a bare Kind or an Error. Use errors.As
// with a pointer to Error to read the first instance's Code, Name, Message
// and Attrs.
// Standard wrappers and joined errors follow Go's traversal rules.
// Applications choose logging fields, protocol mappings and handling policies.
package errkind
