package source

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	incusAPI "github.com/lxc/incus/v6/shared/api"
	incusTLS "github.com/lxc/incus/v6/shared/tls"

	internalAPI "github.com/FuturFusion/migration-manager/internal/api"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type InternalNSXSource struct {
	InternalSource            `yaml:",inline"`
	InternalNSXSourceSpecific `yaml:",inline"`
}

type InternalNSXSourceSpecific struct {
	internalAPI.NSXSourceProperties `yaml:",inline"`

	c *http.Client
}

// PaginationResponse handles paginated API responses.
type PaginationResponse struct {
	Results []any  `json:"results"`
	Count   int    `json:"result_count"`
	Cursor  string `json:"cursor"`
}

// VersionResponse is returned from the NSX API when fetching the version.
type VersionResponse struct {
	Version string `json:"product_version"`
	Build   string `json:"product_build_number"`
}

func NewInternalNSXSourceFrom(apiSource api.Source) (*InternalNSXSource, error) {
	if apiSource.SourceType != api.SOURCETYPE_NSX {
		return nil, errors.New("Source is not of type NSX")
	}

	var connProperties api.VMwareProperties
	err := json.Unmarshal(apiSource.Properties, &connProperties)
	if err != nil {
		return nil, err
	}

	return &InternalNSXSource{
		InternalSource: InternalSource{
			Source: apiSource,
		},
		InternalNSXSourceSpecific: InternalNSXSourceSpecific{
			NSXSourceProperties: internalAPI.NSXSourceProperties{
				VMwareProperties: connProperties,
			},
		},
	}, nil
}

// Connect verifies the NSX server cert against the trusted fingerprint, and fetches the NSX Manager version.
func (s *InternalNSXSource) Connect(ctx context.Context) error {
	if s.isConnected {
		return fmt.Errorf("Already connected to endpoint %q", s.Endpoint)
	}

	endpointURL, err := url.Parse(s.Endpoint)
	if err != nil {
		return err
	}

	if endpointURL == nil {
		return fmt.Errorf("Invalid endpoint: %s", s.Endpoint)
	}

	var serverCert *x509.Certificate
	if len(s.ServerCertificate) > 0 {
		serverCert, err = x509.ParseCertificate(s.ServerCertificate)
		if err != nil {
			return err
		}
	}

	// Unset TLS server certificate if configured but doesn't match the provided trusted fingerprint.
	if serverCert != nil && incusTLS.CertFingerprint(serverCert) != strings.ToLower(strings.ReplaceAll(s.TrustedServerCertificateFingerprint, ":", "")) {
		serverCert = nil
	}

	// Create an empty client, and populate it with a trusted server cert.
	s.c = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{}}}
	if serverCert != nil {
		certpool := x509.NewCertPool()
		certpool.AddCert(serverCert)
		s.c.Transport.(*http.Transport).TLSClientConfig.RootCAs = certpool
	}

	// Get the version information for this NSX manager.
	b, err := s.httpGet(ctx, "api/v1/node/version")
	if err != nil {
		return err
	}

	var version VersionResponse
	err = json.Unmarshal(b, &version)
	if err != nil {
		return err
	}

	s.version = version.Version
	s.isConnected = true

	return nil
}

func (s *InternalNSXSource) FetchSourceData(ctx context.Context) error {
	if !s.isConnected {
		return fmt.Errorf("Already connected to endpoint %q", s.Endpoint)
	}

	segments, err := s.GetSegments(ctx, true)
	if err != nil {
		return fmt.Errorf("Failed to get segments for source %q: %w", s.Name, err)
	}

	computeManagers, err := s.GetComputeManagers(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get compute managers for source %q: %w", s.Name, err)
	}

	edgeNodes, err := s.GetEdgeNodes(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get edge nodes for source %q: %w", s.Name, err)
	}

	policies, err := s.GetSecurityPolicies(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get security policies for source %q: %w", s.Name, err)
	}

	s.NSXSourceProperties = internalAPI.NSXSourceProperties{
		VMwareProperties: s.VMwareProperties,
		ComputeManagers:  computeManagers,
		Segments:         segments,
		EdgeNodes:        edgeNodes,
		Policies:         policies,
	}

	return nil
}

// DoBasicConnectivityCheck performs a connectivity check and verifies the server certificate against the trusted fingerprint.
func (s *InternalNSXSource) DoBasicConnectivityCheck() (api.ExternalConnectivityStatus, *x509.Certificate) {
	status, cert := util.DoBasicConnectivityCheck(s.Endpoint, s.TrustedServerCertificateFingerprint)
	if cert != nil && s.ServerCertificate == nil {
		// We got an untrusted certificate; if one hasn't already been set, add it to this source.
		s.ServerCertificate = cert.Raw
	}

	return status, cert
}

// GetSegment fetches a segment by its segment path, and includes any VMs from the supplied list if their VIFs use a logical port on the segment.
func (s *InternalNSXSource) GetSegment(ctx context.Context, segmentPath string, vms []internalAPI.NSXVirtualMachine) (*internalAPI.NSXSegment, error) {
	if !s.isConnected {
		return nil, fmt.Errorf("Already connected to endpoint %q", s.Endpoint)
	}

	b, err := s.httpGet(ctx, "policy/api/v1"+segmentPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to get segment %q for %q: %w", segmentPath, s.Name, err)
	}

	vmsBySegmentPort := map[string][]internalAPI.NSXVirtualMachine{}
	for _, vm := range vms {
		for _, vif := range vm.VIFs {
			if vmsBySegmentPort[vif.SegmentPortID] == nil {
				vmsBySegmentPort[vif.SegmentPortID] = []internalAPI.NSXVirtualMachine{}
			}

			vmsBySegmentPort[vif.SegmentPortID] = append(vmsBySegmentPort[vif.SegmentPortID], vm)
		}
	}

	var segment internalAPI.NSXSegment
	err = json.Unmarshal(b, &segment)
	if err != nil {
		return nil, fmt.Errorf("Segment data from %q is invalid: %w", s.Name, err)
	}

	return s.AddSegmentData(ctx, &segment, vms)
}

// AddSegmentData populates the segment data with firewall rules, and adds the VMs from the given set that exist on the segment.
func (s *InternalNSXSource) AddSegmentData(ctx context.Context, segment *internalAPI.NSXSegment, vms []internalAPI.NSXVirtualMachine) (*internalAPI.NSXSegment, error) {
	if !s.isConnected {
		return nil, fmt.Errorf("Already connected to endpoint %q", s.Endpoint)
	}

	vmsBySegmentPort := map[string][]internalAPI.NSXVirtualMachine{}
	for _, vm := range vms {
		for _, vif := range vm.VIFs {
			if vmsBySegmentPort[vif.SegmentPortID] == nil {
				vmsBySegmentPort[vif.SegmentPortID] = []internalAPI.NSXVirtualMachine{}
			}

			vmsBySegmentPort[vif.SegmentPortID] = append(vmsBySegmentPort[vif.SegmentPortID], vm)
		}
	}

	if segment.ConnectivityPath != "" {
		path := "policy/api/v1" + segment.ConnectivityPath + "/gateway-firewall"
		allResults, err := s.paginatedGet(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("Failed to get gateway firewall rules for %q: %w", s.Name, err)
		}

		if len(allResults) != 1 {
			return nil, fmt.Errorf("Expected only one gateway police, got %d", len(allResults))
		}

		var obj internalAPI.NSXGatewayPolicy
		entryJSON, err := json.Marshal(allResults[0])
		if err != nil {
			return nil, fmt.Errorf("Failed to parse gateway firewall rule responses for %q: %w", s.Name, err)
		}

		err = json.Unmarshal(entryJSON, &obj)
		if err != nil {
			return nil, fmt.Errorf("Gateway firewall rule data from %q is invalid: %w", s.Name, err)
		}

		segment.Rules = obj
	}

	allResults, err := s.paginatedGet(ctx, "policy/api/v1/infra/segments/"+segment.ID+"/ports")
	if err != nil {
		return nil, fmt.Errorf("Failed to get segment ports for %q: %w", s.Name, err)
	}

	segment.VMs = []internalAPI.NSXVirtualMachine{}
	vmMap := map[uuid.UUID]internalAPI.NSXVirtualMachine{}
	for _, entry := range allResults {
		var obj internalAPI.NSXSegmentPort
		entryJSON, err := json.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse segment port responses for %q: %w", s.Name, err)
		}

		err = json.Unmarshal(entryJSON, &obj)
		if err != nil {
			return nil, fmt.Errorf("Segment port data from %q is invalid: %w", s.Name, err)
		}

		vms, ok := vmsBySegmentPort[obj.Attachment.ID]
		if ok {
			for _, vm := range vms {
				_, ok := vmMap[vm.UUID]
				if !ok {
					vmMap[vm.UUID] = vm
					segment.VMs = append(segment.VMs, vm)
				}
			}
		}
	}

	return segment, nil
}

// GetSegments fetches all segments, their VMs, and gateway policies.
func (s *InternalNSXSource) GetSegments(ctx context.Context, populateData bool) ([]internalAPI.NSXSegment, error) {
	if !s.isConnected {
		return nil, fmt.Errorf("Already connected to endpoint %q", s.Endpoint)
	}

	allResults, err := s.paginatedGet(ctx, "policy/api/v1/infra/segments")
	if err != nil {
		return nil, fmt.Errorf("Failed to get segments for %q: %w", s.Name, err)
	}

	var vms []internalAPI.NSXVirtualMachine
	if populateData {
		vms, err = s.GetVMs(ctx)
		if err != nil {
			return nil, err
		}
	}

	vmsBySegmentPort := map[string][]internalAPI.NSXVirtualMachine{}
	for _, vm := range vms {
		for _, vif := range vm.VIFs {
			if vmsBySegmentPort[vif.SegmentPortID] == nil {
				vmsBySegmentPort[vif.SegmentPortID] = []internalAPI.NSXVirtualMachine{}
			}

			vmsBySegmentPort[vif.SegmentPortID] = append(vmsBySegmentPort[vif.SegmentPortID], vm)
		}
	}

	allSegments := make([]internalAPI.NSXSegment, 0, len(allResults))
	for _, entry := range allResults {
		var segment internalAPI.NSXSegment
		entryJSON, err := json.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse segment responses for %q: %w", s.Name, err)
		}

		err = json.Unmarshal(entryJSON, &segment)
		if err != nil {
			return nil, fmt.Errorf("Segment data from %q is invalid: %w", s.Name, err)
		}

		if populateData {
			_, err = s.AddSegmentData(ctx, &segment, vms)
			if err != nil {
				return nil, err
			}
		}

		allSegments = append(allSegments, segment)
	}

	return allSegments, nil
}

// GetVMs fetches all VMs and their VIFs.
func (s *InternalNSXSource) GetVMs(ctx context.Context) ([]internalAPI.NSXVirtualMachine, error) {
	if !s.isConnected {
		return nil, fmt.Errorf("Already connected to endpoint %q", s.Endpoint)
	}

	allVMResults, err := s.paginatedGet(ctx, "api/v1/fabric/virtual-machines")
	if err != nil {
		return nil, fmt.Errorf("Failed to get VMs for %q: %w", s.Name, err)
	}

	allVIFResults, err := s.paginatedGet(ctx, "api/v1/fabric/vifs")
	if err != nil {
		return nil, fmt.Errorf("Failed to get VIFs for %q: %w", s.Name, err)
	}

	allVMs := make([]internalAPI.NSXVirtualMachine, len(allVMResults))
	for i, entry := range allVMResults {
		var obj internalAPI.NSXVirtualMachine
		data, err := json.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse VM responses for %q: %w", s.Name, err)
		}

		err = json.Unmarshal(data, &obj)
		if err != nil {
			return nil, fmt.Errorf("VM data from %q is invalid: %w", s.Name, err)
		}

		allVMs[i] = obj
	}

	netMap := map[uuid.UUID][]internalAPI.NSXVIF{}
	for _, entry := range allVIFResults {
		var obj internalAPI.NSXVIF
		data, err := json.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse VIF responses for %q: %w", s.Name, err)
		}

		err = json.Unmarshal(data, &obj)
		if err != nil {
			return nil, fmt.Errorf("VIF data from %q is invalid: %w", s.Name, err)
		}

		if netMap[obj.UUID] == nil {
			netMap[obj.UUID] = []internalAPI.NSXVIF{}
		}

		netMap[obj.UUID] = append(netMap[obj.UUID], obj)
	}

	for i, vm := range allVMs {
		if netMap[vm.UUID] != nil {
			allVMs[i].VIFs = netMap[vm.UUID]
		}
	}

	return allVMs, nil
}

// GetSecurityPolicies fetches all security policies for all domains.
func (s *InternalNSXSource) GetSecurityPolicies(ctx context.Context) ([]internalAPI.NSXSecurityPolicy, error) {
	if !s.isConnected {
		return nil, fmt.Errorf("Already connected to endpoint %q", s.Endpoint)
	}

	policies := []internalAPI.NSXSecurityPolicy{}
	allResults, err := s.paginatedGet(ctx, "policy/api/v1/infra/domains")
	if err != nil {
		return nil, fmt.Errorf("Failed to get domains for %q: %w", s.Name, err)
	}

	for _, entry := range allResults {
		var obj internalAPI.NSXDomain
		entryJSON, err := json.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse domain responses for %q: %w", s.Name, err)
		}

		err = json.Unmarshal(entryJSON, &obj)
		if err != nil {
			return nil, fmt.Errorf("Domain data from %q is invalid: %w", s.Name, err)
		}

		path := "policy/api/v1" + obj.Path + "/security-policies"
		allResults, err := s.paginatedGet(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("Failed to get security policies for %q: %w", s.Name, err)
		}

		policy := internalAPI.NSXSecurityPolicy{Domain: obj.Name, Rules: []internalAPI.NSXRule{}}
		for _, entry := range allResults {
			var obj internalAPI.NSXRule
			entryJSON, err := json.Marshal(entry)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse security policie responses for %q: %w", s.Name, err)
			}

			err = json.Unmarshal(entryJSON, &obj)
			if err != nil {
				return nil, fmt.Errorf("Security policy data from %q is invalid: %w", s.Name, err)
			}

			policy.Rules = append(policy.Rules, obj)
		}

		policies = append(policies, policy)
	}

	return policies, nil
}

// GetComputeManagers gets all compute managers registered with the NSX Manager.
func (s *InternalNSXSource) GetComputeManagers(ctx context.Context) ([]internalAPI.NSXComputeManager, error) {
	if !s.isConnected {
		return nil, fmt.Errorf("Already connected to endpoint %q", s.Endpoint)
	}

	allResults, err := s.paginatedGet(ctx, "api/v1/fabric/compute-managers")
	if err != nil {
		return nil, fmt.Errorf("Failed to get compute managers for %q: %w", s.Name, err)
	}

	allComputeManagers := make([]internalAPI.NSXComputeManager, 0, len(allResults))
	for _, computeEntry := range allResults {
		var comp internalAPI.NSXComputeManager
		compJSON, err := json.Marshal(computeEntry)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse compute manager responses for %q: %w", s.Name, err)
		}

		err = json.Unmarshal(compJSON, &comp)
		if err != nil {
			return nil, fmt.Errorf("Compute manager data from %q is invalid: %w", s.Name, err)
		}

		allComputeManagers = append(allComputeManagers, comp)
	}

	return allComputeManagers, nil
}

// GetTransportZone fetches a transport zone by its UUID.
func (s *InternalNSXSource) GetTransportZone(ctx context.Context, zoneUUID uuid.UUID) (*internalAPI.NSXTransportZone, error) {
	if !s.isConnected {
		return nil, fmt.Errorf("Already connected to endpoint %q", s.Endpoint)
	}

	data, err := s.httpGet(ctx, "api/v1/transport-zones/"+zoneUUID.String())
	if err != nil {
		return nil, fmt.Errorf("Failed to get transport zones for %q: %w", s.Name, err)
	}

	var zone internalAPI.NSXTransportZone
	err = json.Unmarshal(data, &zone)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse transport zone responses for %q: %w", s.Name, err)
	}

	zone.UUID = zoneUUID
	return &zone, nil
}

// GetEdgeNodes fetches all edge transport nodes of the NSX Manager.
func (s *InternalNSXSource) GetEdgeNodes(ctx context.Context) ([]internalAPI.NSXEdgeTransportNode, error) {
	if !s.isConnected {
		return nil, fmt.Errorf("Already connected to endpoint %q", s.Endpoint)
	}

	allResults, err := s.paginatedGet(ctx, "api/v1/transport-nodes")
	if err != nil {
		return nil, fmt.Errorf("Failed to get edge transport nodes for %q: %w", s.Name, err)
	}

	nodes := []internalAPI.NSXEdgeTransportNode{}
	for _, entry := range allResults {
		var obj internalAPI.NSXEdgeTransportNode
		data, err := json.Marshal(entry)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse edge transport node responses for %q: %w", s.Name, err)
		}

		err = json.Unmarshal(data, &obj)
		if err != nil {
			return nil, fmt.Errorf("Edge transport node data from %q is invalid: %w", s.Name, err)
		}

		if obj.Info.Type != "EdgeNode" {
			continue
		}

		for i, sw := range obj.HostSwitches.Switches {
			if sw.IPPool.UUID != uuid.Nil {
				data, err := s.httpGet(ctx, "api/v1/pools/ip-pools/"+sw.IPPool.UUID.String())
				if err != nil {
					return nil, fmt.Errorf("Failed to get IP pools for %q: %w", s.Name, err)
				}

				var pool internalAPI.NSXIPPool
				err = json.Unmarshal(data, &pool)
				if err != nil {
					return nil, fmt.Errorf("Failed to parse IP pool responses for %q: %w", s.Name, err)
				}

				pool.UUID = sw.IPPool.UUID
				obj.HostSwitches.Switches[i].IPPool = pool
			}

			for j, zoneID := range sw.TransportZones {
				zone, err := s.GetTransportZone(ctx, zoneID.UUID)
				if err != nil {
					return nil, err
				}

				obj.HostSwitches.Switches[i].TransportZones[j] = *zone
			}
		}

		nodes = append(nodes, obj)
	}

	return nodes, nil
}

func (s *InternalNSXSource) makeRequest(ctx context.Context, method string, path string, header http.Header, content []byte) (*http.Response, error) {
	urlPath := s.Endpoint + path
	req, err := http.NewRequestWithContext(ctx, method, urlPath, bytes.NewBuffer(content))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(s.Username, s.Password)
	if header != nil {
		req.Header = header
	} else {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.c.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// httpGet makes a single http request to the given path, with the given optional header and contents, and returns the response body.
func (s *InternalNSXSource) httpGet(ctx context.Context, path string) ([]byte, error) {
	resp, err := s.makeRequest(ctx, http.MethodGet, "/"+path, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("error fetching data from %s: %w", path, err)
	}

	defer func() { _ = resp.Body.Close() }()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API call failed: URL=%s, Status=%s, Response=%s", path, resp.Status, string(body))
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body from %s: %w", path, err)
	}

	return body, nil
}

// paginatedGet makes a request to a paginated API, iterating until all results have been gathered. Returns a list of response bodies.
func (s *InternalNSXSource) paginatedGet(ctx context.Context, path string) ([]any, error) {
	var allResults []any
	var cursor string
	pageNum := 1

	nextPath := incusAPI.NewURL()
	nextPath.Path(strings.Split(path, "/")...)
	for {
		// Add cursor to query parameters if present
		if cursor != "" {
			nextPath = nextPath.WithQuery("cursor", cursor)
		}

		resp, err := s.makeRequest(ctx, http.MethodGet, nextPath.String(), nil, nil)
		if err != nil {
			return nil, fmt.Errorf("Error fetching data from %q: %w", nextPath.String(), err)
		}

		defer func() { _ = resp.Body.Close() }() //nolint:revive

		// Check the response status code
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("NSX API call failed: URL=%q, Status=%q, Response=%q", nextPath.String(), resp.Status, string(body))
		}

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Error reading response body from %q: %w", nextPath.String(), err)
		}

		// Unmarshal the JSON response
		var paginatedResp PaginationResponse
		err = json.Unmarshal(body, &paginatedResp)
		if err != nil {
			return nil, fmt.Errorf("Error unmarshaling JSON from %q: %w", nextPath.String(), err)
		}

		// Append the results to allResults
		allResults = append(allResults, paginatedResp.Results...)

		// Check if there's a cursor to continue pagination
		if paginatedResp.Cursor == "" {
			break
		}

		err = resp.Body.Close()
		if err != nil {
			return nil, err
		}

		cursor = paginatedResp.Cursor
		pageNum++
	}

	return allResults, nil
}
