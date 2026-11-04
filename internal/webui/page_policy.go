package webui

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
)

// scriptNoncePlaceholder stands in a page template's script tags. It is filled
// in the same pass as the page's values, so a value holding its text cannot
// obtain the nonce.
const scriptNoncePlaceholder = "__ROTARI_SCRIPT_NONCE__"

func scriptNonceAttribute(nonce string) string {
	if nonce == "" {
		return ""
	}
	return ` nonce="` + nonce + `"`
}

// pagePolicy is the Content-Security-Policy of a live page. Only the page's
// own scripts run, which leaves a value that became markup unable to run script
// or send data elsewhere. Scripts are matched by nonce rather than 'self', since
// a launched service answers on the same origin. Styles stay inline: sizes and
// colours that depend on data are set in style attributes.
func pagePolicy(nonce string) string {
	return "default-src 'none'; script-src 'nonce-" + nonce + "'; style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; font-src 'self' data:; media-src 'self' blob:; frame-src 'self'; " +
		"connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'self'; object-src 'none'"
}

// setPagePolicy sets the policy of one page response and returns its nonce.
func setPagePolicy(writer http.ResponseWriter) string {
	var raw [18]byte
	_, _ = rand.Read(raw[:]) // crypto/rand.Read does not fail; it aborts the program instead.
	nonce := base64.RawURLEncoding.EncodeToString(raw[:])
	writer.Header().Set("Content-Security-Policy", pagePolicy(nonce))
	return nonce
}
