package infra

import (
	"bufio"
	"strings"
	"time"

	"github.com/xhanio/errors"
	"github.com/xhanio/framingo/pkg/utils/cmdutil"
)

var (
	Debug        bool
	StartTime    time.Time
	SerialNumber string
	Hostname     string
	Timezone     *time.Location = time.Local
	ConfigDir    string
	EnvPrefix    string
)

func GetTimezone() (string, error) {
	cmd := cmdutil.New("timedatectl", []string{"show"})
	err := cmd.Start()
	if err != nil {
		return "", errors.Wrap(err)
	}
	scanner := bufio.NewScanner(strings.NewReader(cmd.Output()))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Timezone=") {
			tz := strings.TrimPrefix(line, "Timezone=")
			return tz, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", errors.Wrap(err)
	}
	// no timezone data found in the timedatectl output, keep the original timezone
	return "", errors.Newf("timezone data not found")
}

func LoadTimezone(tz string) error {
	var err error
	Timezone, err = time.LoadLocation(tz)
	if err != nil {
		return errors.Wrap(err)
	}
	return nil
}
