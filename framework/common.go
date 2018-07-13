package framework

import (
	"regexp"
	"strings"
)

var spaceReg, _ = regexp.Compile("\\s{2,}")

func CompressSpace(str string) string {
	return spaceReg.ReplaceAllString(str, " ")
}

func ReplaceStr(ori string, old string, newFunc func() string) string {
	for {
		res := strings.Replace(ori, old, newFunc(), -1)
		if ori == res {
			return ori
		} else {
			ori = res
		}
	}
}

const (
	defaultServerName  = "fasthttp"
	defaultUserAgent   = "fasthttp"
	defaultContentType = "text/plain; charset=utf-8"
)

const (
	Slash               = "/"
	SlashSlash          = "//"
	SlashDotDot         = "/.."
	SlashDotSlash       = "/./"
	SlashDotDotSlash    = "/../"
	CRLF                = "\r\n"
	HTTP                = "http"
	HTTPS               = "https"
	HTTP11              = "HTTP/1.1"
	ColonSlashSlash     = "://"
	ColonSpace          = ": "
	GMT                 = "GMT"
	ResponseContinue    = "HTTP/1.1 100 Continue\r\n\r\n"
	GET                 = "GET"
	HEAD                = "HEAD"
	POST                = "POST"
	PUT                 = "PUT"
	DELETE              = "DELETE"
	OPTIONS             = "OPTIONS"
	EXPECT              = "EXPECT"
	Connection          = "Connection"
	ContentLength       = "Content-Length"
	ContentType         = "Content-Type"
	Date                = "Date"
	Host                = "Host"
	Referer             = "Referer"
	ServerHeader        = "Server"
	TransferEncoding    = "Transfer-Encoding"
	ContentEncoding     = "Content-Encoding"
	AcceptEncoding      = "Accept-Encoding"
	UserAgent           = "User-Agent"
	Cookie              = "Cookie"
	SetCookie           = "Set-Cookie"
	Location            = "Location"
	IfModifiedSince     = "If-Modified-Since"
	LastModified        = "Last-Modified"
	AcceptRanges        = "Accept-Ranges"
	Range               = "Range"
	ContentRange        = "Content-Range"
	CookieExpires       = "expires"
	CookieDomain        = "domain"
	CookiePath          = "Path"
	CookieHTTPOnly      = "HttpOnly"
	CookieSecure        = "secure"
	HttpClose           = "close"
	Gzip                = "gzip"
	Deflate             = "deflate"
	KeepAlive           = "keep-alive"
	KeepAliveCamelCase  = "Keep-Alive"
	Upgrade             = "Upgrade"
	Chunked             = "chunked"
	Identity            = "identity"
	PostArgsContentType = "application/x-www-form-urlencoded"
	MultipartFormData   = "multipart/form-data"
	Boundary            = "boundary"
	Bytes               = "bytes"
	TextSlash           = "text/"
	ApplicationSlash    = "application/"
)

const (
	ApplicationJson   = "application/json; charset=utf-8"
	Css               = "text/css; charset=utf-8"
	Plain             = "text/plain; charset=utf-8"
	Html              = "text/html; charset=utf-8"
	Jpeg              = "image/jpeg"
	Js                = "application/x-javascript; charset=utf-8"
	Pdf               = "application/pdf"
	Png               = "image/png"
	Svg               = "image/svg+xml"
	Xml               = "text/xml; charset=utf-8"
	ApplicationFont   = "application/x-font-woff"
	ApplicationStream = "application/octet-stream"
)

const (
	AccessControlAllowOrigin  = "Access-Control-Allow-Origin"
	AccessControlAllowMethods = "Access-Control-Allow-Methods"
	AccessControlAllowHeaders = "Access-Control-Allow-Headers"
	METHODS                   = "POST,GET,OPTIONS,DELETE"

)
