// Package osutil answers questions about the operating system the process
// runs on. Everything here is a query, not process-wide state: a caller that
// needs an answer asks for it, and decides what to do when the system has
// none to give.
package osutil

import (
	"bufio"
	"strings"
	"time"

	"github.com/xhanio/errors"

	"github.com/xhanio/framingo/pkg/utils/cmdutil"
)

// Timezone is the timezone the system is configured with, as timedatectl
// reports it on a systemd host. It errors where timedatectl is missing or
// reports none. A caller that only needs local time uses time.Local, which Go
// resolves from TZ and /etc/localtime on its own; this is for the system's
// named setting.
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
