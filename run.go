package main

import (
	"net/http"
	"strings"

	"github.com/Luzifer/gh-private-dl/privatehub"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	sparta "github.com/mweagle/Sparta"
	"github.com/spf13/cobra"
)

var executionPort string

func init() {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run an HTTP server to serve this locally",
		RunE:  runLocally,
	}

	cmd.Flags().StringVar(&executionPort, "listen", ":3000", "IP/Port to listen on")

	sparta.CommandLineOptions.Root.AddCommand(cmd)
}

func runLocally(cmd *cobra.Command, args []string) error {
	r := mux.NewRouter()
	r.HandleFunc("/{user}/{repo}/releases/download/{version}/{binary}", handleLocalExecution)
	r.HandleFunc("/status", func(res http.ResponseWriter, r *http.Request) { res.WriteHeader(http.StatusNoContent) })

	log.Printf("Starting local webserver on %s", executionPort)
	return http.ListenAndServe(executionPort, r)
}

func handleLocalExecution(res http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	_, githubToken, ok := r.BasicAuth()
	if !ok || githubToken == "" {
		http.Error(res, "You need to provide HTTP basic auth", http.StatusBadRequest)
		return
	}

	dlURL, err := privatehub.GetDownloadURL(
		strings.Join([]string{vars["user"], vars["repo"]}, "/"),
		vars["version"],
		vars["binary"],
		githubToken,
	)

	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, r, dlURL, http.StatusFound)
}
