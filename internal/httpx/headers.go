package httpx

import "net/http"

var hop = map[string]bool{"Connection": true, "Keep-Alive": true, "Proxy-Authenticate": true, "Proxy-Authorization": true, "TE": true, "Trailer": true, "Transfer-Encoding": true, "Upgrade": true, "Host": true, "Content-Length": true}

func CopySafe(dst, src http.Header) {
	for k, vv := range src {
		if hop[http.CanonicalHeaderKey(k)] {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func CopyRequest(dst, src http.Header) {
	for _, k := range []string{"Content-Type", "Accept", "X-Request-ID"} {
		if v := src.Get(k); v != "" {
			dst.Set(k, v)
		}
	}
}
