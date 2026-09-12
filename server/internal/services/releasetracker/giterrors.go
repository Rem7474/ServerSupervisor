package releasetracker

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/gitprovider"
)

// gitProviderI18nKey maps a gitprovider.APIError's HTTP status to the
// matching apperr catalog key (+ interpolation params for the generic
// fallback, which embeds the raw status).
func gitProviderI18nKey(status int) (string, map[string]string) {
	switch status {
	case http.StatusUnauthorized:
		return apperr.CodeGitProviderUnauthorized, nil
	case http.StatusForbidden:
		return apperr.CodeGitProviderRateLimited, nil
	case http.StatusNotFound:
		return apperr.CodeGitProviderNotFound, nil
	default:
		return apperr.CodeGitProviderError, map[string]string{"status": strconv.Itoa(status)}
	}
}

// gitProviderHTTPError converts a gitprovider fetch error into an
// apperr.BadGateway carrying the matching I18nKey, so respondError renders it
// in the requesting browser's language. Non-APIError causes (network
// failures, decode errors, …) fall back to their own Error() text untouched.
func gitProviderHTTPError(err error) *apperr.Error {
	var apiErr *gitprovider.APIError
	if errors.As(err, &apiErr) {
		key, params := gitProviderI18nKey(apiErr.Status)
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
		key, params := gitProviderI18nKey(apiErr.Status)
		return apperr.GetMessage(key, "fr", params)
	}
	return err.Error()
}
