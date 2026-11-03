package source

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	incusTLS "github.com/lxc/incus/v7/shared/tls"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

const vropsAPIPath = "/suite-api/api"

var vropsMonitoringStatKeys = []string{
	"cpu|usage_average", "mem|usage_average",
	"disk|read_average", "disk|write_average",
	"guestfilesystem|usage_total", "guestfilesystem|capacity_total",
	"storage|usage_average",
	"storage|commandsAveraged_average", "storage|totalLatency_average",
	"net|received_average", "net|transmitted_average",
}

type vropsToken struct {
	Token    string `json:"token"`
	Validity int64  `json:"validity"`
}

type vropsResource struct {
	Identifier  string `json:"identifier"`
	ResourceKey struct {
		AdapterKind  string `json:"adapterKindKey"`
		ResourceKind string `json:"resourceKindKey"`
		Identifiers  []struct {
			Type struct {
				Name string `json:"name"`
			} `json:"identifierType"`
			Value string `json:"value"`
		} `json:"resourceIdentifiers"`
	} `json:"resourceKey"`
	Health       string `json:"resourceHealth"`
	StatusStates []struct {
		Status string `json:"resourceStatus"`
	} `json:"resourceStatusStates"`
}

// InternalVROpsSource connects to the Aria Operations suite API.
type InternalVROpsSource struct {
	InternalSource
	api.VROpsProperties
	client      *http.Client
	token       string
	tokenExpiry time.Time
}

func NewInternalVROpsSourceFrom(src api.Source) (*InternalVROpsSource, error) {
	if src.SourceType != api.SOURCETYPE_VROPS {
		return nil, fmt.Errorf("Source is not of type vROps")
	}

	var props api.VROpsProperties
	err := json.Unmarshal(src.Properties, &props)
	if err != nil {
		return nil, err
	}

	props.SetDefaults()
	props.TrustedServerCACertificates = append(props.TrustedServerCACertificates, util.SystemCACertificates()...)
	return &InternalVROpsSource{
		InternalSource:  InternalSource{Source: src, connectionTimeout: props.ConnectionTimeout.Duration},
		VROpsProperties: props,
	}, nil
}

func (s *InternalVROpsSource) DoBasicConnectivityCheck() (api.ExternalConnectivityStatus, *x509.Certificate) {
	status, cert := util.DoBasicConnectivityCheck(s.Endpoint, s.TrustedServerCertificateFingerprint, s.TrustedServerCACertificates)
	if cert != nil {
		s.ServerCertificate = cert.Raw
	}

	return status, cert
}

func (s *InternalVROpsSource) Connect(ctx context.Context) error {
	if s.isConnected {
		return fmt.Errorf("Already connected to endpoint %q", s.Endpoint)
	}

	endpoint, err := url.Parse(s.Endpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.User != nil || (endpoint.Path != "" && endpoint.Path != "/") {
		return fmt.Errorf("vROps endpoint must be an HTTPS URL without credentials, query or fragment")
	}

	ctx, cancel := context.WithTimeout(ctx, s.ConnectionTimeout.Duration)
	defer cancel()
	var serverCert *x509.Certificate
	if len(s.ServerCertificate) > 0 {
		serverCert, err = x509.ParseCertificate(s.ServerCertificate)
		if err != nil {
			return err
		}

		if incusTLS.CertFingerprint(serverCert) != strings.ToLower(strings.ReplaceAll(s.TrustedServerCertificateFingerprint, ":", "")) {
			serverCert = nil
		}
	}

	tlsConfig, err := util.TLSClientConfig(serverCert, s.TrustedServerCACertificates)
	if err != nil {
		return err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	s.client = &http.Client{
		Transport:     transport,
		Timeout:       s.ConnectionTimeout.Duration,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}

	return s.acquireToken(ctx)
}

func (s *InternalVROpsSource) apiURL(path string) string {
	return strings.TrimRight(s.Endpoint, "/") + vropsAPIPath + path
}

func (s *InternalVROpsSource) acquireToken(ctx context.Context) error {
	body, err := json.Marshal(map[string]string{"username": s.Username, "password": s.Password, "authSource": s.AuthSource})
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL("/auth/token/acquire"), bytes.NewReader(body))
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return fmt.Errorf("vROps authentication: not authorized")
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("vROps authentication failed with status %s", response.Status)
	}

	var token vropsToken
	err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&token)
	if err != nil {
		return err
	}

	if token.Token == "" {
		return fmt.Errorf("vROps authentication response did not contain a token")
	}

	s.token = token.Token
	s.tokenExpiry = time.Time{}
	if token.Validity > 0 {
		s.tokenExpiry = time.UnixMilli(token.Validity)
	}

	s.isConnected = true
	return nil
}

type vropsResourcePage struct {
	ResourceList []vropsResource `json:"resourceList"`
	PageInfo     *struct {
		TotalCount *int `json:"totalCount"`
		PageSize   int  `json:"pageSize"`
	} `json:"pageInfo"`
}

func (s *InternalVROpsSource) getResources(ctx context.Context, page int, pageSize int) (vropsResourcePage, error) {
	var response vropsResourcePage

	path := fmt.Sprintf("/resources?adapterKind=VMWARE&resourceKind=VirtualMachine&page=%d&pageSize=%d", page, pageSize)
	err := s.getJSON(ctx, path, &response)
	return response, err
}

type vropsStat struct {
	Key struct {
		Key string `json:"key"`
	} `json:"statKey"`
	Data       []*float64 `json:"data"`
	Timestamps []int64    `json:"timestamps"`
}

type vropsStatList struct {
	Stat []vropsStat `json:"stat"`
}

type vropsResourceStats struct {
	ResourceID string        `json:"resourceId"`
	StatList   vropsStatList `json:"stat-list"`
	Stats      vropsStatList `json:"stats"`
}

func (r vropsResourceStats) statistics() []vropsStat {
	if r.StatList.Stat != nil {
		return r.StatList.Stat
	}

	return r.Stats.Stat
}

type vropsStatsResponse struct {
	Values []vropsResourceStats `json:"values"`
}

func (s *InternalVROpsSource) getLatestStats(ctx context.Context, resourceID string) (map[string]float64, map[string]time.Time, error) {
	query := url.Values{"statKey": vropsMonitoringStatKeys, "maxSamples": {"1"}, "currentOnly": {"true"}, "metrics": {"true"}}
	path := "/resources/" + url.PathEscape(resourceID) + "/stats/latest?" + query.Encode()
	var response vropsStatsResponse

	err := s.getJSON(ctx, path, &response)
	if err != nil {
		return nil, nil, err
	}

	metrics := map[string]float64{}
	sampleTimes := map[string]time.Time{}
	for _, resource := range response.Values {
		if resource.ResourceID != resourceID {
			continue
		}

		for _, stat := range resource.statistics() {
			key := stat.Key.Key
			if !slices.Contains(vropsMonitoringStatKeys, key) {
				continue
			}

			if len(stat.Data) == 0 || len(stat.Data) != len(stat.Timestamps) {
				continue
			}

			latest := 0
			for i, timestamp := range stat.Timestamps {
				if timestamp > stat.Timestamps[latest] {
					latest = i
				}
			}

			value := stat.Data[latest]
			if value == nil || *value < 0 || stat.Timestamps[latest] <= 0 {
				continue
			}

			sampleTime := time.UnixMilli(stat.Timestamps[latest]).UTC()
			// JSON timestamps cannot represent years after 9999.
			if sampleTime.Year() > 9999 {
				continue
			}

			metrics[key] = *value
			sampleTimes[key] = sampleTime
		}
	}

	return metrics, sampleTimes, nil
}

func (s *InternalVROpsSource) getJSON(ctx context.Context, path string, result any) error {
	if !s.isConnected {
		return fmt.Errorf("not connected to vROps")
	}

	for attempt := 0; attempt < 2; attempt++ {
		if !s.tokenExpiry.IsZero() && !time.Now().Before(s.tokenExpiry) {
			err := s.acquireToken(ctx)
			if err != nil {
				return err
			}
		}

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL(path), nil)
		if err != nil {
			return err
		}

		request.Header.Set("Accept", "application/json")
		request.Header.Set("Authorization", "vRealizeOpsToken "+s.token)
		response, err := s.client.Do(request)
		if err != nil {
			return err
		}

		if response.StatusCode == http.StatusUnauthorized && attempt == 0 {
			response.Body.Close()
			err = s.acquireToken(ctx)
			if err != nil {
				return err
			}

			continue
		}

		if response.StatusCode != http.StatusOK {
			requestErr := s.requestError(response)
			response.Body.Close()
			return requestErr
		}

		err = json.NewDecoder(io.LimitReader(response.Body, 32<<20)).Decode(result)
		response.Body.Close()
		return err
	}

	return fmt.Errorf("vROps authentication retry exhausted")
}

func (s *InternalVROpsSource) requestError(response *http.Response) error {
	err := fmt.Errorf("vROps request failed with status %s", response.Status)
	var detail struct {
		Message string `json:"message"`
	}

	decodeErr := json.NewDecoder(io.LimitReader(response.Body, 16<<10)).Decode(&detail)
	if decodeErr != nil || detail.Message == "" {
		return err
	}

	message := detail.Message
	secrets := []string{s.Username, s.Password, s.token}
	slices.SortFunc(secrets, func(a string, b string) int { return len(b) - len(a) })
	replacements := make([]string, 0, 2*len(secrets))
	for _, secret := range secrets {
		if secret != "" {
			replacements = append(replacements, secret, "[redacted]")
		}
	}

	message = strings.NewReplacer(replacements...).Replace(message)
	message = strings.Join(strings.Fields(message), " ")
	runes := []rune(message)
	if len(runes) > 1024 {
		message = string(runes[:1024]) + "..."
	}

	return fmt.Errorf("%w: %s", err, message)
}

// Close releases idle connections after collection.
func (s *InternalVROpsSource) Close() {
	if s.client != nil {
		s.client.CloseIdleConnections()
	}

	s.isConnected = false
}
