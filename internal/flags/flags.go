package flags

import "github.com/alecthomas/kong"

// API api related flags passing in env variables.
type API struct {
	Version              kong.VersionFlag
	AthenaDatabase       string `help:"The athena database name" env:"ATHENA_DATABASE"`
	AthenaCatalog        string `help:"The athena catalog name" env:"ATHENA_CATALOG"`
	AthenaWorkgroup      string `help:"The athena workgroup name" env:"ATHENA_WORKGROUP"`
	QueryTemplatesBucket string `help:"The s3 bucket name containing the query templates" env:"QUERY_TEMPLATES_BUCKET"`
}
