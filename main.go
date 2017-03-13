package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Luzifer/gh-private-dl/privatehub"
	"github.com/Sirupsen/logrus"
	sparta "github.com/mweagle/Sparta"
)

const (
	httpRequestTimeout = 5 * time.Second
)

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

func handleLambdaGithubDownload(event *json.RawMessage, context *sparta.LambdaContext, w http.ResponseWriter, logger *logrus.Logger) {
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

	dlURL, err := privatehub.GetDownloadURL(
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
	lambdaFn := sparta.NewLambda(sparta.IAMRoleDefinition{}, handleLambdaGithubDownload, &sparta.LambdaFunctionOptions{
		Timeout: 30,
	})
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
