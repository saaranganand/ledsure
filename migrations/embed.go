// embeds the goose SQL files into the ledsured binary
package migrations

import "embed"

// every *.sql file embedded at build time
// the embed directive can't reach outside this dir, so this file isn't in internal/store
//
//go:embed *.sql
var FS embed.FS
