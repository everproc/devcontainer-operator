package controller

import (
	"net/url"
	"regexp"
)

func ParseGitUrl(u string) (string, error) {
	var gitReg = regexp.MustCompile(`^([a-zA-Z0-9_]+)@([a-zA-Z0-9._-]+):(.*)$`)
	m := gitReg.FindStringSubmatch(u)
	var host string
	if m != nil {
		host = m[2]
	} else {
		parsed, err := url.Parse(u)
		if err != nil {
			return "", err
		}
		host = parsed.String()
	}
	return host, nil
}
