package consul

import (
	"net/url"
)

func GetURL(rawURL string, path string, token string) (string, error) {
	URL, err := url.Parse(rawURL)
	URL.Path = path
	if token != "" {
		query := URL.Query()
		query.Set("token", token)
		URL.RawQuery = query.Encode()
	}
	if err != nil {
		return "", err
	}
	return URL.String(), nil
}
