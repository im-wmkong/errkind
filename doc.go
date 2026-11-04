// Package errkind provides business error identities, instances and cause chains.
//
// Define registers a stable Kind in the default Registry. NewRegistry isolates
// definitions when applications or tests need independent error catalogs.
// Kind and Error are interfaces backed by private implementations. Registered
// identities are immutable. Use Kind to define and construct business errors
// and as an errors.Is target. Return instances created by Kind.New or Kind.Wrap
// from business operations, using New("") when no message is needed. Instances
// carry literal messages and attributes constructed by With. Wrap with a nil
// cause returns nil.
//
// At application boundaries, use a single errors.As call with a pointer to
// Error to read the first instance's Kind, Code, Name, Message and attributes.
// Attr reads a key from that instance without searching the cause; Attrs returns
// a shallow copy of all its attributes.
// Standard wrappers and joined errors follow Go's traversal rules.
// Applications choose logging fields, protocol mappings and handling policies.
//
// A Kind can be used as an error, and errors.As with a *Kind target extracts an
// identity. A registered Kind is not an Error instance. When no instance is found,
// applications can still record the full error text. Identity and instance queries
// may select different nodes; read an instance's identity through Error.Kind
// instead of combining the results of separate queries.
package errkind
