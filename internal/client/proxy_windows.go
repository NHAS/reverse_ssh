//go:build windows

package client

import (
	"fmt"

	"github.com/NHAS/reverse_ssh/pkg/wauth"
)

func AdditionalHeaders(proxy string, req []string) []string {
	req = append(req, fmt.Sprintf("Proxy-Authorization: %s", wauth.GetAuthorizationHeader(proxy)))
	return req
}
