package client

import (
	"context"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog"
	"io"
	"net/http"
	"time"
)

type Client struct {
	r   *resty.Client
	log zerolog.Logger
}
type Result struct {
	Status  int
	Header  http.Header
	Body    []byte
	Err     error
	Timeout bool
}

func New(debug bool, log zerolog.Logger) *Client {
	r := resty.New().SetDebug(debug).SetTimeout(0)
	return &Client{r, log}
}
func (c *Client) Do(ctx context.Context, method, url, token, requestID string, h http.Header, body io.Reader) Result {
	start := time.Now()
	req := c.r.R().SetContext(ctx).SetHeader("Authorization", token).SetHeader("X-Request-ID", requestID)
	for _, k := range []string{"Content-Type", "Accept"} {
		if v := h.Get(k); v != "" {
			req.SetHeader(k, v)
		}
	}
	if body != nil {
		req.SetBody(body)
	}
	resp, err := req.Execute(method, url)
	c.log.Debug().Str("method", method).Str("url", url).Int("status", status(resp)).Dur("duration", time.Since(start)).Err(err).Msg("amocrm request")
	if err != nil {
		return Result{Err: err, Timeout: ctx.Err() != nil}
	}
	return Result{Status: resp.StatusCode(), Header: resp.Header(), Body: resp.Body()}
}
func status(r *resty.Response) int {
	if r == nil {
		return 0
	}
	return r.StatusCode()
}
