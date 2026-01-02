// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package gerr

import (
	"errors"
	"net/http"
)

// HttpError provides ready to use HTTP code and payload for HTTP handlers.
func HttpError(err error) (int, error) {
	// Return a standardized error response
	var gErr *Error
	if !errors.As(err, &gErr) {
		// If not an gerr.Error, wrap it and return internal server error
		gErr = wrap(err, ServerErr, "internal server error")
	}
	return statusCode(gErr.Code), gErr
}

// statusCode deduce the HTTP status code from an ErrorType.
func statusCode(errType Code) int {
	switch errType {
	case Invalid:
		return http.StatusBadRequest
	case NotFound:
		return http.StatusNotFound
	case Timeout:
		return http.StatusRequestTimeout
	case UserAbort:
		return http.StatusNoContent
	case ConfigErr, InferErr, ServerErr:
		fallthrough
	default:
		return http.StatusInternalServerError
	}
}
