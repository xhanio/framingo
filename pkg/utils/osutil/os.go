// Package osutil queries the host OS; it holds no process-wide state.
package osutil

import (
	"bufio"
	"strings"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/cmdutil"
)

// Timezone reads timedatectl and errors where it is missing or reports none; for local time use time.Local.
func Timezone() (*time.Location, error) {
	name, err := timezoneName()
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, errors.Wrapf(err, "timezone %s", name)
	}
	return loc, nil
}

func timezoneName() (string, error) {
	c := cmdutil.New("timedatectl", []string{"show"})
	if err := c.Run(); err != nil {
		return "", errors.Wrap(err)
	}
	scanner := bufio.NewScanner(strings.NewReader(c.Output()))
	for scanner.Scan() {
		if name, ok := strings.CutPrefix(scanner.Text(), "Timezone="); ok {
			return name, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", errors.Wrap(err)
	}
	return "", errors.NotFound.Newf("timedatectl reports no timezone")
}
