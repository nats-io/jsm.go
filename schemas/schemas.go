package schemas

import (
	"embed"
)

//go:embed jetstream
//go:embed server
//go:embed micro

var schemas embed.FS

func Load(schema string) ([]byte, error) {
	return schemas.ReadFile(schema)
}
