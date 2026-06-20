package api

import "github.com/xhanio/framingo/pkg/types/common"

type Encoding string

const (
	EncodingDeflate Encoding = "deflate"
)

const (
	ContextKeyRequestInfo  = common.ContextKeyAPIRequestInfo
	ContextKeyResponseInfo = common.ContextKeyAPIResponseInfo
	ContextKeyError        = common.ContextKeyAPIError
	ContextKeyCredential   = common.ContextKeyCredential
	ContextKeySession      = common.ContextKeySession
	ContextKeyTrace        = common.ContextKeyTrace
	ContextKeyDB           = common.ContextKeyDB
	ContextKeyLogger       = common.ContextKeyLogger

	CookiesKeySession = "JSESSIONID"

	HeaderKeyTrace        = "X-TRACE-ID"
	HeaderKeyFile         = "X-FILE-ID"
	HeaderKeyJob          = "X-JOB-ID"
	HeaderKeySession      = "X-SESSION-ID"
	HeaderKeyAPIToken     = "X-API-KEY"
	HeaderKeyAgentID      = "X-AGENT-ID"
	HeaderKeyClientCert   = "X-Ssl-Certificate"        // from nginx proxy_set_header $ssl_client_escaped_cert
	HeaderKeyClientVerify = "X-Ssl-Certificate-Verify" // from nginx proxy_set_header $ssl_client_verify

	QueryParamSession = "sid"
	QueryParamJob     = "job"

	LabelKeySession   = "session"
	LabelKeyNamespace = "organization"
	LabelKeyUsername  = "username"
)
