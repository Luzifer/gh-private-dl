package privatehub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	httpRequestTimeout = 5 * time.Second
)

var (
	AssetNotFound   = errors.New("Did not find given asset in release")
	ReleaseNotFound = errors.New("Did not find given release")
)

type ghRelease struct {
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"assets"`
}

func GetDownloadURL(repo, version, filename, ghToken string) (string, error) {
	path := fmt.Sprintf("/repos/%s/releases/tags/%s", repo, version)
	if version == "latest" {
		path = fmt.Sprintf("/repos/%s/releases/latest", repo)
	}
	u := fmt.Sprintf("https://api.github.com%s", path)

	req, _ := http.NewRequest("GET", u, nil)
	req.SetBasicAuth("auth", ghToken)
	ctx, cancel := context.WithTimeout(context.Background(), httpRequestTimeout)
	defer cancel()

	httpClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", ReleaseNotFound
	}

	r := &ghRelease{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		return "", err
	}

	var dlURL string
	for _, ass := range r.Assets {
		if ass.Name != filename {
			continue
		}

		dlURL = ass.URL
	}

	if dlURL == "" {
		return "", AssetNotFound
	}

	req, _ = http.NewRequest("HEAD", dlURL, nil)
	req.Header.Set("Accept", "application/octet-stream")
	req.SetBasicAuth("auth", ghToken)

	ctx, cancel = context.WithTimeout(context.Background(), httpRequestTimeout)
	defer cancel()

	resp, err = httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return resp.Header.Get("Location"), nil
}
