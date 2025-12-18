package nacos

import (
	"net/url"
)

/**
 * @Description: Build Nacos API URL address
 * @Param rawURL: Base URL address
 * @Param path: API path
 * @Param query: Query parameters
 * @Return string: Complete API URL address
 * @Return error: Error message
 */
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
