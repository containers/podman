package probs

import (
	"fmt"
	"net/http"

	"github.com/letsencrypt/boulder/identifier"
)

// Error types that can be used in ACME payloads
const (
	ConnectionProblem            = ProblemType("connection")
	MalformedProblem             = ProblemType("malformed")
	ServerInternalProblem        = ProblemType("serverInternal")
	TLSProblem                   = ProblemType("tls")
	UnauthorizedProblem          = ProblemType("unauthorized")
	RateLimitedProblem           = ProblemType("rateLimited")
	BadNonceProblem              = ProblemType("badNonce")
	InvalidEmailProblem          = ProblemType("invalidEmail")
	RejectedIdentifierProblem    = ProblemType("rejectedIdentifier")
	AccountDoesNotExistProblem   = ProblemType("accountDoesNotExist")
	CAAProblem                   = ProblemType("caa")
	DNSProblem                   = ProblemType("dns")
	AlreadyRevokedProblem        = ProblemType("alreadyRevoked")
	OrderNotReadyProblem         = ProblemType("orderNotReady")
	BadSignatureAlgorithmProblem = ProblemType("badSignatureAlgorithm")
	BadPublicKeyProblem          = ProblemType("badPublicKey")
	BadRevocationReasonProblem   = ProblemType("badRevocationReason")
	BadCSRProblem                = ProblemType("badCSR")

	V1ErrorNS = "urn:acme:error:"
	V2ErrorNS = "urn:ietf:params:acme:error:"
)

// ProblemType defines the error types in the ACME protocol
type ProblemType string

// ProblemDetails objects represent problem documents
// https://tools.ietf.org/html/draft-ietf-appsawg-http-problem-00
type ProblemDetails struct {
	Type   ProblemType `json:"type,omitempty"`
	Detail string      `json:"detail,omitempty"`
	// HTTPStatus is the HTTP status code the ProblemDetails should probably be sent
	// as.
	HTTPStatus int `json:"status,omitempty"`
	// SubProblems are optional additional per-identifier problems. See
	// RFC 8555 Section 6.7.1: https://tools.ietf.org/html/rfc8555#section-6.7.1
	SubProblems []SubProblemDetails `json:"subproblems,omitempty"`
}

// SubProblemDetails represents sub-problems specific to an identifier that are
// related to a top-level ProblemDetails.
// See RFC 8555 Section 6.7.1: https://tools.ietf.org/html/rfc8555#section-6.7.1
type SubProblemDetails struct {
	ProblemDetails
	Identifier identifier.ACMEIdentifier `json:"identifier"`
}

func (pd *ProblemDetails) Error() string {
	return fmt.Sprintf("%s :: %s", pd.Type, pd.Detail)
}

// WithSubProblems returns a new ProblemsDetails instance created by adding the
// provided subProbs to the existing ProblemsDetail.
func (pd *ProblemDetails) WithSubProblems(subProbs []SubProblemDetails) *ProblemDetails {
	return &ProblemDetails{
		Type:        pd.Type,
		Detail:      pd.Detail,
		HTTPStatus:  pd.HTTPStatus,
		SubProblems: append(pd.SubProblems, subProbs...),
	}
}

// statusTooManyRequests is the HTTP status code meant for rate limiting
// errors. It's not currently in the net/http library so we add it here.
const statusTooManyRequests = 429

// ProblemDetailsToStatusCode inspects the given ProblemDetails to figure out
// what HTTP status code it should represent. It should only be used by the WFE
// but is included in this package because of its reliance on ProblemTypes.
func ProblemDetailsToStatusCode(prob *ProblemDetails) int {
	if prob.HTTPStatus != 0 {
		return prob.HTTPStatus
	}
	switch prob.Type {
	case
		ConnectionProblem,
		MalformedProblem,
		BadSignatureAlgorithmProblem,
		BadPublicKeyProblem,
		TLSProblem,
		BadNonceProblem,
		InvalidEmailProblem,
		RejectedIdentifierProblem,
		AccountDoesNotExistProblem,
		BadRevocationReasonProblem:
		return http.StatusBadRequest
	case ServerInternalProblem:
		return http.StatusInternalServerError
	case
		UnauthorizedProblem,
		CAAProblem:
		return http.StatusForbidden
	case RateLimitedProblem:
		return statusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

// BadNonce returns a ProblemDetails with a BadNonceProblem and a 400 Bad
// Request status code.
func BadNonce(detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       BadNonceProblem,
		Detail:     detail,
		HTTPStatus: http.StatusBadRequest,
	}
}

// RejectedIdentifier returns a ProblemDetails with a RejectedIdentifierProblem and a 400 Bad
// Request status code.
func RejectedIdentifier(detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       RejectedIdentifierProblem,
		Detail:     detail,
		HTTPStatus: http.StatusBadRequest,
	}
}

// Conflict returns a ProblemDetails with a MalformedProblem and a 409 Conflict
// status code.
func Conflict(detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       MalformedProblem,
		Detail:     detail,
		HTTPStatus: http.StatusConflict,
	}
}

// AlreadyRevoked returns a ProblemDetails with a AlreadyRevokedProblem and a 400 Bad
// Request status code.
func AlreadyRevoked(detail string, a ...interface{}) *ProblemDetails {
	return &ProblemDetails{
		Type:       AlreadyRevokedProblem,
		Detail:     fmt.Sprintf(detail, a...),
		HTTPStatus: http.StatusBadRequest,
	}
}

// Malformed returns a ProblemDetails with a MalformedProblem and a 400 Bad
// Request status code.
func Malformed(detail string, args ...interface{}) *ProblemDetails {
	if len(args) > 0 {
		detail = fmt.Sprintf(detail, args...)
	}
	return &ProblemDetails{
		Type:       MalformedProblem,
		Detail:     detail,
		HTTPStatus: http.StatusBadRequest,
	}
}

// Canceled returns a ProblemDetails with a MalformedProblem and a 408 Request
// Timeout status code.
func Canceled(detail string, args ...interface{}) *ProblemDetails {
	if len(args) > 0 {
		detail = fmt.Sprintf(detail, args...)
	}
	return &ProblemDetails{
		Type:       MalformedProblem,
		Detail:     detail,
		HTTPStatus: http.StatusRequestTimeout,
	}
}

// BadSignatureAlgorithm returns a ProblemDetails with a BadSignatureAlgorithmProblem
// and a 400 Bad Request status code.
func BadSignatureAlgorithm(detail string, a ...interface{}) *ProblemDetails {
	return &ProblemDetails{
		Type:       BadSignatureAlgorithmProblem,
		Detail:     fmt.Sprintf(detail, a...),
		HTTPStatus: http.StatusBadRequest,
	}
}

// BadPublicKey returns a ProblemDetails with a BadPublicKeyProblem and a 400 Bad
// Request status code.
func BadPublicKey(detail string, a ...interface{}) *ProblemDetails {
	return &ProblemDetails{
		Type:       BadPublicKeyProblem,
		Detail:     fmt.Sprintf(detail, a...),
		HTTPStatus: http.StatusBadRequest,
	}
}

// NotFound returns a ProblemDetails with a MalformedProblem and a 404 Not Found
// status code.
func NotFound(detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       MalformedProblem,
		Detail:     detail,
		HTTPStatus: http.StatusNotFound,
	}
}

// ServerInternal returns a ProblemDetails with a ServerInternalProblem and a
// 500 Internal Server Failure status code.
func ServerInternal(detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       ServerInternalProblem,
		Detail:     detail,
		HTTPStatus: http.StatusInternalServerError,
	}
}

// Unauthorized returns a ProblemDetails with an UnauthorizedProblem and a 403
// Forbidden status code.
func Unauthorized(detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       UnauthorizedProblem,
		Detail:     detail,
		HTTPStatus: http.StatusForbidden,
	}
}

// MethodNotAllowed returns a ProblemDetails representing a disallowed HTTP
// method error.
func MethodNotAllowed() *ProblemDetails {
	return &ProblemDetails{
		Type:       MalformedProblem,
		Detail:     "Method not allowed",
		HTTPStatus: http.StatusMethodNotAllowed,
	}
}

// ContentLengthRequired returns a ProblemDetails representing a missing
// Content-Length header error
func ContentLengthRequired() *ProblemDetails {
	return &ProblemDetails{
		Type:       MalformedProblem,
		Detail:     "missing Content-Length header",
		HTTPStatus: http.StatusLengthRequired,
	}
}

// InvalidContentType returns a ProblemDetails suitable for a missing
// ContentType header, or an incorrect ContentType header
func InvalidContentType(detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       MalformedProblem,
		Detail:     detail,
		HTTPStatus: http.StatusUnsupportedMediaType,
	}
}

// InvalidEmail returns a ProblemDetails representing an invalid email address
// error
func InvalidEmail(detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       InvalidEmailProblem,
		Detail:     detail,
		HTTPStatus: http.StatusBadRequest,
	}
}

// ConnectionFailure returns a ProblemDetails representing a ConnectionProblem
// error
func ConnectionFailure(detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       ConnectionProblem,
		Detail:     detail,
		HTTPStatus: http.StatusBadRequest,
	}
}

// RateLimited returns a ProblemDetails representing a RateLimitedProblem error
func RateLimited(detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       RateLimitedProblem,
		Detail:     detail,
		HTTPStatus: statusTooManyRequests,
	}
}

// TLSError returns a ProblemDetails representing a TLSProblem error
func TLSError(detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       TLSProblem,
		Detail:     detail,
		HTTPStatus: http.StatusBadRequest,
	}
}

// AccountDoesNotExist returns a ProblemDetails representing an
// AccountDoesNotExistProblem error
func AccountDoesNotExist(detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       AccountDoesNotExistProblem,
		Detail:     detail,
		HTTPStatus: http.StatusBadRequest,
	}
}

// CAA returns a ProblemDetails representing a CAAProblem
func CAA(detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       CAAProblem,
		Detail:     detail,
		HTTPStatus: http.StatusForbidden,
	}
}

// DNS returns a ProblemDetails representing a DNSProblem
func DNS(detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       DNSProblem,
		Detail:     detail,
		HTTPStatus: http.StatusBadRequest,
	}
}

// OrderNotReady returns a ProblemDetails representing a OrderNotReadyProblem
func OrderNotReady(detail string, a ...interface{}) *ProblemDetails {
	return &ProblemDetails{
		Type:       OrderNotReadyProblem,
		Detail:     fmt.Sprintf(detail, a...),
		HTTPStatus: http.StatusForbidden,
	}
}

// BadRevocationReason returns a ProblemDetails representing
// a BadRevocationReasonProblem
func BadRevocationReason(detail string, a ...interface{}) *ProblemDetails {
	return &ProblemDetails{
		Type:       BadRevocationReasonProblem,
		Detail:     fmt.Sprintf(detail, a...),
		HTTPStatus: http.StatusBadRequest,
	}
}

// BadCSR returns a ProblemDetails representing a BadCSRProblem.
func BadCSR(detail string, a ...interface{}) *ProblemDetails {
	return &ProblemDetails{
		Type:       BadCSRProblem,
		Detail:     fmt.Sprintf(detail, a...),
		HTTPStatus: http.StatusBadRequest,
	}
}
