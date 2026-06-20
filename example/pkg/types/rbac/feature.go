package rbac

const (
	FeatureBasic = "basic"
)

var Features map[string][]string = map[string][]string{
	FeatureBasic: PermissionsAll, // no control on demo
}
