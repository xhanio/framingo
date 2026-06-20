package entity

const (
	BackupAssetTypeFiles  = "files"
	BackupAssetTypeConfig = "config"
	BackupAssetTypeCmdb   = "cmdb"

	BackupDatabaseTypeFull    = "full_backup" // Full backup of the database
	BackupDatabaseTypePartial = "partial_backup"
)

type BackupConfig struct {
	FilesPath   string
	ConfigsPath string
}

type Asset struct {
	Type   string `yaml:"type"`   // "config" or "files"
	Dest   string `yaml:"dest"`   // system path of the config or files
	Source string `yaml:"source"` // file path in the tar file package
}

type Database struct {
	Type          string    `yaml:"type"` // "full_backup" or "partial_backup"
	Source        string    `yaml:"source"`
	WithTables    *[]string `yaml:"tables,omitempty"`        // Only used when type is "partial"
	WithoutTables *[]string `yaml:"withoutTables,omitempty"` // Only used when type is "partial"
}

type BackupMetadata struct {
	ProductName   string   `yaml:"product_name"`
	SystemVersion string   `yaml:"system_version"`
	Assets        []Asset  `yaml:"assets"`
	Database      Database `yaml:"database"`
}

type BackupDatabaseOptions struct {
	Type          string
	WithoutTables []string
}
