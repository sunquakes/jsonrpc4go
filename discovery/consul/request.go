package consul

import (
	"net/url"
)

/**
 * @Description: Build Consul API URL
 * @Param rawURL: Base URL address
 * @Param path: API path
 * @Param token: Authentication token (optional)
 * @Return string: Complete API URL address
 * @Return error: Error message
 */
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
