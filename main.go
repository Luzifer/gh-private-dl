package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	sparta "github.com/mweagle/Sparta"
)

type ghRelease struct {
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"assets"`
}

func extractGithubToken(lambdaEvent sparta.APIGatewayLambdaJSONEvent) (string, error) {
	if hdr, ok := lambdaEvent.Headers["Authorization"]; ok {
		auth := strings.SplitN(hdr, " ", 2)
		if len(auth) != 2 || auth[0] != "Basic" {
			return "", errors.New("You need to provide HTTP basic auth")
		}

		payload, err := base64.StdEncoding.DecodeString(auth[1])
		if err != nil {
			return "", errors.New("You need to provide HTTP basic auth")
		}

		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 {
			return "", errors.New("You need to provide HTTP basic auth")
		}

		return pair[1], nil
	}
	return "", errors.New("You need to provide HTTP basic auth")
}

func getDownloadURL(repo, version, filename, ghToken string) (string, error) {
	path := fmt.Sprintf("/repos/%s/releases/tags/%s", repo, version)
	if version == "latest" {
		path = fmt.Sprintf("/repos/%s/releases/latest", repo)
	}
	u := fmt.Sprintf("https://api.github.com%s", path)

	req, _ := http.NewRequest("GET", u, nil)
	req.SetBasicAuth("auth", ghToken)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
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
		return "", errors.New("Did not find given release")
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
		return "", errors.New("Did not find given filename in release")
	}

	req, _ = http.NewRequest("HEAD", dlURL, nil)
	req.Header.Set("Accept", "application/octet-stream")
	req.SetBasicAuth("auth", ghToken)

	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err = httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return resp.Header.Get("Location"), nil
}

func handleGithubDownload(event *json.RawMessage, context *sparta.LambdaContext, w http.ResponseWriter, logger *logrus.Logger) {
	var lambdaEvent sparta.APIGatewayLambdaJSONEvent
	err := json.Unmarshal([]byte(*event), &lambdaEvent)
	if err != nil {
		logger.Error("Failed to unmarshal event data: ", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	githubToken, err := extractGithubToken(lambdaEvent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dlURL, err := getDownloadURL(
		strings.Join([]string{lambdaEvent.PathParams["user"], lambdaEvent.PathParams["repo"]}, "/"),
		lambdaEvent.PathParams["version"],
		lambdaEvent.PathParams["binary"],
		githubToken,
	)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := sparta.ArbitraryJSONObject{
		"location": dlURL,
	}

	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusFound)
	json.NewEncoder(w).Encode(response)
}

func main() {
	var lambdaFunctions []*sparta.LambdaAWSInfo
	lambdaFn := sparta.NewLambda(sparta.IAMRoleDefinition{}, handleGithubDownload, nil)
	lambdaFunctions = append(lambdaFunctions, lambdaFn)

	stage := sparta.NewStage("prod")
	apiGateway := sparta.NewAPIGateway("GH-PrivateDL-API", stage)

	// https://github.com/Luzifer/vault2env/releases/download/v0.6.1/vault2env_linux_amd64
	apiGatewayResource, _ := apiGateway.NewResource("/{user}/{repo}/releases/download/{version}/{binary}", lambdaFn)
	method, err := apiGatewayResource.NewMethod("GET", http.StatusFound)
	if err != nil {
		panic(err)
	}
	method.Parameters["method.request.path.user"] = true
	method.Parameters["method.request.path.repo"] = true
	method.Parameters["method.request.path.version"] = true
	method.Parameters["method.request.path.binary"] = true
	method.Parameters["method.request.header.authorization"] = true

	method.Responses[http.StatusFound].Parameters = map[string]bool{
		"method.response.header.Location": true,
	}

	method.Integration.Responses[http.StatusFound].Parameters["method.response.header.Location"] = "integration.response.body.location"

	// Deploy it
	sparta.MainEx("GH-PrivateDL",
		"Lambda application for downloading private GitHub assets",
		lambdaFunctions,
		apiGateway,
		nil,
		nil)
}
