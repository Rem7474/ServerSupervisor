package releasetracker

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/gitprovider"
)

// gitProviderI18nKey maps a gitprovider.APIError's HTTP status to the
// matching apperr catalog key (+ interpolation params: the provider display
// name, and the raw status for the generic fallback).
func gitProviderI18nKey(apiErr *gitprovider.APIError) (string, map[string]string) {
	provider := apiErr.Provider
	if provider == "" {
		provider = "GitHub"
	}
	params := map[string]string{"provider": provider}
	switch apiErr.Status {
	case http.StatusUnauthorized:
		return apperr.CodeGitProviderUnauthorized, params
	case http.StatusForbidden:
		return apperr.CodeGitProviderRateLimited, params
	case http.StatusNotFound:
		return apperr.CodeGitProviderNotFound, params
	default:
		params["status"] = strconv.Itoa(apiErr.Status)
		return apperr.CodeGitProviderError, params
	}
}

// gitProviderHTTPError converts a gitprovider fetch error into an
// apperr.BadGateway carrying the matching I18nKey, so respondError renders it
// in the requesting browser's language. Non-APIError causes (network
// failures, decode errors, …) fall back to their own Error() text untouched.
func gitProviderHTTPError(err error) *apperr.Error {
	var apiErr *gitprovider.APIError
	if errors.As(err, &apiErr) {
		key, params := gitProviderI18nKey(apiErr)
		return apperr.BadGateway(apperr.GetMessage(key, "en", params)).I18n(key, params)
	}
	return apperr.BadGateway(err.Error())
}

// gitProviderPollError renders a gitprovider fetch error for the
// asynchronous, DB-persisted "last error" field on a tracker (poller.go) —
// there is no requester/Accept-Language for a background job, so it resolves
// to French, matching this app's long-standing default. Non-APIError causes
// keep their own Error() text.
func gitProviderPollError(err error) string {
	var apiErr *gitprovider.APIError
	if errors.As(err, &apiErr) {
		key, params := gitProviderI18nKey(apiErr)
		return apperr.GetMessage(key, "fr", params)
	}
	return err.Error()
}
