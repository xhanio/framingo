package cronutil

import (
	"github.com/robfig/cron/v3"
	"github.com/xhanio/errors"
)

var Parser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

func Validate(spec string) error {
	if _, err := Parser.Parse(spec); err != nil {
		return errors.InvalidArgument.Wrapf(err, "invalid schedule %q", spec)
	}
	return nil
}
