package main

// TaskFailureReason enum failure reason codes when task failed
type TaskFailureReason string

// WARNING: MUST keep in sync with TaskFailureReason enum defined in GraphQL
// https://github.com/veritone/core-graphql-server/blob/master/schema/schema.graphql//L4485
// Any value defined here but not in GraphQL will cause any GraphQL call using that value to fail
// NOTE: "unknown" vs "other".  Use the former is when failure cause cannot be identified.  Use the
// latter when failure cause is known but there is no failure reason code for it (yet), in which
// case supply as much detail as possible in FailureMessage (free text) which we'll use to evaluate
// adding new failure reason codes in future.
const (
	// The engine encountered an unexpected internal error.
	FailureReasonInternalError TaskFailureReason = "internal_error"

	// The cause of the failure could not be determined.
	FailureReasonUnknown TaskFailureReason = "unknown"

	// The engine attempted to download
	// content from a URL provided in the task payload and
	// received a 404.
	FailureReasonURLNotFound TaskFailureReason = "url_not_found"

	// The engine attempted to download
	// content from a URL provided in the task payload and
	// received a 401 or 403.
	FailureReasonURLNotAllowed TaskFailureReason = "url_not_allowed"

	// The engine attempted to download
	// content from a URL provided in the task payload and
	// the download timed out
	FailureReasonURLTimeout TaskFailureReason = "url_timeout"

	// The engine attempted to download
	// content from a URL provided in the task payload and
	// the connection was refused.
	FailureReasonURLConnectionRefused TaskFailureReason = "url_connection_refused"

	// The engine attempted to download content from a URL
	// provided in the task payload an received an error.
	FailureReasonURLError TaskFailureReason = "url_error"

	// The input to the engine was incompatible with the engine
	// requirements. For example, an input media file had an
	// unsupported MIME type or the file was empty.
	FailureReasonInvalidData TaskFailureReason = "invalid_data"

	// An engine operation was subject to rate limiting.
	FailureReasonRateLimited TaskFailureReason = "rate_limited"

	// The engine received an authorization error from the Veritone API.
	FailureReasonAPINotAllowed TaskFailureReason = "api_not_allowed"

	// The engine received an authentication error from the Veritone API using
	// the token provided in the task payload.
	FailureReasonAPIAuthenticationError TaskFailureReason = "api_authentication_error"

	// The engine received a "not found" error from the Veritone API on
	// a required object.
	FailureReasonAPINotFound TaskFailureReason = "api_not_found"

	// An unexpected error was received from the Veritone API, such as
	// HTTP 500, HTTP 502, or an `internal_error` error.
	FailureReasonAPIError TaskFailureReason = "api_error"

	// The engine could not write temporary files to disk for processing
	// due to disk space full or other system error.
	FailureReasonFileWriteError TaskFailureReason = "file_write_error"

	// The engine encountered a missing binary dependency or configuration,
	// such as a missing executable or package or incompatible hardware.
	FailureReasonSystemDependencyMissing TaskFailureReason = "system_dependency_missing"

	// The engine encountered an operating system, hardware, or other
	// system-level error.
	FailureReasonSystemError TaskFailureReason = "system_error"

	// The engine failed to send heartbeat or Edge didn't receive it in time
	FailureReasonHeartbeatTimeout TaskFailureReason = "heartbeat_timeout"

	// The engine failed to send chunk result, or Edge didn't receive it in time
	FailureReasonChunkTimeout TaskFailureReason = "chunk_timeout"

	// The error cause is known, but could not be mapped to a `TaskFailureReason`
	// value. The `failureMessage` input field should contain details.
	FailureReasonOther TaskFailureReason = "other"
)
