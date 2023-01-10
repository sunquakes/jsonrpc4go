package nacos

import (
	"net/url"
)

func GetURL(rawURL string, path string, query map[string]string) (string, error) {
	URL, err := url.Parse(rawURL)
	URL.Path = path
	q := URL.Query()
	if query != nil {
		for k, v := range query {
			q.Set(k, v)
		}
		URL.RawQuery = q.Encode()
	}
	if err != nil {
		return "", err
	}
	return URL.String(), nil
}
