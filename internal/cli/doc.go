// Package cli is the command line front end: it reads one batch, asks the
// translator for a candidate, validates it with the translation rules, and
// renders the result.
//
// It never writes a locale file. Publishing a locale bundle needs a
// human-reviewed diff, so -publish without -dry-run is refused with status 4.
package cli
