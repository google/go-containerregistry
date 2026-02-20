package transport

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Mirror struct {
	OriginUrl       string
	MirrorEndpoints []MirrorEndpoint
}

type MirrorEndpoint struct {
	Endpoint string
	Secure   bool
}
type mirrorTransport struct {
	inner   http.RoundTripper
	mirrors []Mirror
}

var _ http.RoundTripper = (*mirrorTransport)(nil)

func NewWithMirrors(inner http.RoundTripper, mirrors []Mirror) http.RoundTripper {
	return &mirrorTransport{
		inner:   inner,
		mirrors: mirrors,
	}
}

func (t *mirrorTransport) RoundTrip(in *http.Request) (out *http.Response, err error) {
	if len(t.mirrors) > 0 {
		for _, mirror := range t.mirrors {
			if isApplicable, err := mirror.isApplicableTo(*in.URL); isApplicable && err == nil {
				for _, endpoint := range mirror.MirrorEndpoints {
					mirroredRequest, err := mirror.useMirrorEndpoint(in, endpoint)
					if err != nil {
						fmt.Printf("ERROR: Request %v: %v\n", mirroredRequest, err)
						continue
					}
					out, err = t.inner.RoundTrip(mirroredRequest)
					if err != nil {
						fmt.Printf("ERROR: Request %v: %v\n", mirroredRequest, err)
						continue
					}
					return out, err
				}
			}
		}
	}
	return t.inner.RoundTrip(in)
}

func (m Mirror) isApplicableTo(url url.URL) (bool, error) {
	mirrorUrl, err := url.Parse(m.OriginUrl)
	if err != nil {
		return false, fmt.Errorf("unable to parse mirror origin url %s: %v", m.OriginUrl, err)
	}
	if strings.Contains(url.Host, mirrorUrl.Host) || strings.Contains(url.Path, mirrorUrl.Path) {
		fmt.Printf("INFO: Request %v: mirror %v matches\n", url, m)
		return true, nil
	}
	return false, nil
}

func (m Mirror) useMirrorEndpoint(in *http.Request, mirrorEndpoint MirrorEndpoint) (*http.Request, error) {
	mirrorUrl, err := url.Parse(m.OriginUrl)
	if err != nil {
		return in, fmt.Errorf("unable to parse mirror origin url %s: %v", m.OriginUrl, err)
	}
	mirrorEndpointUrl, err := url.Parse(mirrorEndpoint.Endpoint)
	if err != nil {
		return in, fmt.Errorf("unable to parse mirror endpoint %s: %v", mirrorEndpoint.Endpoint, err)
	}

	mirroredIn := in.Clone(in.Context())
	inURL := in.URL.String()
	inURL = strings.Replace(inURL, mirrorUrl.Host, mirrorEndpointUrl.Host, 1)
	inURL = strings.Replace(inURL, mirrorUrl.Path, mirrorEndpointUrl.Path, 1)
	if in.URL.Scheme == "https" && !mirrorEndpoint.Secure {
		inURL = strings.Replace(inURL, "https", "http", 1)
	}
	if in.URL.Scheme == "http" && mirrorEndpoint.Secure {
		inURL = strings.Replace(inURL, "http", "https", 1)
	}
	mirroredRequestURL, err := url.Parse(inURL)
	if err != nil {
		return in, fmt.Errorf("unable to parse mirror endpoint %s: %v", mirrorEndpoint.Endpoint, err)

	}
	mirroredIn.URL = mirroredRequestURL
	fmt.Printf("using %v as mirror of %v\n", mirroredIn.URL.String(), in.URL.String())
	return mirroredIn, nil
}
