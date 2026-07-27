package migrations

import "embed"

// FS exposes the migration SQL files for tooling (tests, future CLI) that apply
// them without shelling out to the `migrate` binary. Embedded at compile time so
// integration tests run anywhere `go test` runs.
//
//go:embed *.up.sql *.down.sql
var FS embed.FS
