package info

import (
	"fmt"
	"strings"

	"github.com/xhanio/framingo/pkg/utils/sliceutil"
)

var (
	ApplicationName    string
	ApplicationVersion string

	ProductName    string
	ProductModel   string
	ProductVersion string

	ProjectRoot string
	ProjectName string
	ProjectPath string

	GitBranch string
	GitTag    string
	GitCommit string

	BuildVersion string
	BuildType    string
	BuildDate    string
	BuildTime    string

	INJECTION = map[string]*string{
		"ApplicationName":    &ApplicationName,
		"ApplicationVersion": &ApplicationVersion,

		"ProductName":    &ProductName,
		"ProductModel":   &ProductModel,
		"ProductVersion": &ProductVersion,

		"ProjectRoot": &ProjectRoot,
		"ProjectName": &ProjectName,
		"ProjectPath": &ProjectPath,

		"GitBranch": &GitBranch,
		"GitTag":    &GitTag,
		"GitCommit": &GitCommit,

		"BuildVersion": &BuildVersion,
		"BuildType":    &BuildType,
		"BuildDate":    &BuildDate,
		"BuildTime":    &BuildTime,
	}

	build *Build
)

type Build struct {
	ApplicationName    string `json:"application_name"`
	ApplicationVersion string `json:"application_version"`

	ProductName    string `json:"product_name"`
	ProductModel   string `json:"product_model"`
	ProductVersion string `json:"product_version"`

	ProjectRoot string `json:"project_root"`
	ProjectName string `json:"project_name"`
	ProjectPath string `json:"project_path"`

	GitBranch string `json:"git_branch"`
	GitTag    string `json:"git_tag"`
	GitCommit string `json:"git_commit"`

	BuildVersion string `json:"build_version"`
	BuildType    string `json:"build_type"`
	BuildDate    string `json:"build_date"`
	BuildTime    string `json:"build_time"`
}

func GetBuildInfo() Build {
	if build == nil {
		build = &Build{
			ApplicationName:    ApplicationName,
			ApplicationVersion: ApplicationVersion,
			ProductName:        ProductName,
			ProductModel:       ProductModel,
			ProductVersion:     ProductVersion,
			ProjectRoot:        ProjectRoot,
			ProjectName:        ProjectName,
			ProjectPath:        ProjectPath,
			GitBranch:          GitBranch,
			GitTag:             GitTag,
			GitCommit:          GitCommit,
			BuildVersion:       BuildVersion,
			BuildType:          BuildType,
			BuildDate:          BuildDate,
			BuildTime:          BuildTime,
		}
	}
	return *build
}

func Version() string {
	var version []string
	version = append(version, ProductVersion, BuildVersion)
	version = sliceutil.Deduplicate(version...)
	if BuildType != "" {
		version = append(version, fmt.Sprintf("(%s)", BuildType))
	}
	return strings.Join(version, " ")
}
