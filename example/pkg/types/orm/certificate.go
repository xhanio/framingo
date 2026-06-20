package orm

type Certificate struct {
	BaseModel
	Name       string `gorm:"column:name;type:character varying(250);not null"`
	IsCA       bool   `gorm:"column:is_ca;type:boolean;not null"`
	IsLocal    bool   `gorm:"column:is_local;type:boolean;not null"`
	Type       string `gorm:"column:type;type:character varying(250);not null"`
	Source     string `gorm:"column:source;type:character varying(250);not null"`
	Comments   string `gorm:"column:comments;type:character varying(500)"`
	CertBundle []byte `gorm:"column:cert_bundle;type:bytea"`
	RefCount   int    `gorm:"column:ref_count;type:integer"`
}

// TableName specifies the table name for Certificate model.
func (Certificate) TableName() string {
	return "certificates"
}
