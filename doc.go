//Package closed extracts closed types from a package.
//
//A closed type is a type whose valid values
//is a subset of the domain of its underlying type.
//
//It is impossible to extract all closed types,
//but this package attempts to extract all that can be heuristically identified
//by matching common coding conventions.
//However, false positives and negatives are still possible.
package closed
