package utils

import (
	"net/url"
	"os"
)

func GetBaseUrl() string {
	baseUrl := os.Getenv("BASE_URL")

	if baseUrl == "" {
		baseUrl = "http://localhost:6900"
	}

	return baseUrl
}

func GetDomain() string {
	baseUrl := GetBaseUrl()
	parsedUrl, err := url.Parse(baseUrl)
	if err != nil {
		panic(err)
	}
	return parsedUrl.Hostname()
}
