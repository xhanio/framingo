package strutil

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
)

var allLetters = regexp.MustCompile("^[a-zA-Z]+$")

func AllLetters(s string) bool {
	return allLetters.MatchString(s)
}

func Join[T fmt.Stringer](sep string, elements ...T) string {
	var strs []string
	for _, elem := range elements {
		if elem.String() == "" {
			continue
		}
		strs = append(strs, elem.String())
	}
	return strings.Join(strs, sep)
}

func Clean(s string) string {
	return strings.Trim(s, " \n\t")
}

func Random(charset string, length int) string {
	l := len(charset)
	var b strings.Builder
	for range length {
		b.WriteByte(charset[rand.Intn(l)])
	}
	return b.String()
}

func FormatHex(num any, uppercase bool) string {
	format := "%x"
	if uppercase {
		format = "%X"
	}
	hex := fmt.Sprintf(format, num)
	if len(hex)%2 == 1 {
		hex = "0" + hex
	}
	re := regexp.MustCompile("..")
	return strings.TrimRight(re.ReplaceAllString(hex, "$0:"), ":")
}

func PrefixIn(s string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}
