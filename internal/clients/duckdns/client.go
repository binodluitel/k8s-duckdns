package duckdns

import (
	"fmt"
)

// Client for interacting with the DuckDNS service
type Client struct {
	host    string
	verbose bool
}

// ClientOption is a functional option pattern wrapper for configuring the client.
type ClientOption func(*Client) error

// Params defines parameters accepted byr the DuckDNS update request
type Params struct {
	// Clear indicates whether to clear the IPv4 and IPv6 from the DNS record
	Clear bool `json:"clear,omitempty"`

	// Domains is a comma-separated list of domains to update
	Domains string `json:"domains"`

	// IPv4 and IPv6 are optional parameters to specify the IP addresses to update.
	// If not specified, DuckDNS will detect the current IP address. This only works for IPv4 addresses.
	// If both are specified, DuckDNS will update the record with the provided IPs.
	IPv4 string `json:"ipv4,omitempty"`

	// IPv6 is an optional parameter to specify the IPv6 address to update.
	IPv6 string `json:"ipv6,omitempty"`

	// Txt is an optional parameter to specify a TXT record to be associated with the domain.
	Txt string `json:"txt,omitempty"`

	// Token is the authentication token for DuckDNS service.
	Token string `json:"token,omitempty"`
}

// NewClient creates a new DuckDNS Client with the specified protocol, domain, and token
func NewClient(protocol, domain string, options ...ClientOption) (*Client, error) {
	if protocol != "http" && protocol != "https" {
		return nil, fmt.Errorf("invalid protocol %q. Use 'http' or 'https'", protocol)
	}
	if domain == "" {
		return nil, fmt.Errorf("domain cannot be empty")
	}
	if domain[len(domain)-1] == '/' {
		domain = domain[:len(domain)-1] // Remove the trailing slash if present
	}
	c := new(Client)
	c.host = fmt.Sprintf("%s://%s", protocol, domain)
	for _, o := range options {
		if err := o(c); err != nil {
			return nil, fmt.Errorf("error applying client option: %w", err)
		}
	}
	return c, nil
}

// EnableVerbosity returns a ClientOption to enable or disable verbose logging in the Client configuration.
func EnableVerbosity(enable bool) ClientOption {
	return func(c *Client) error {
		c.verbose = enable
		return nil
	}
}

// UpdateURL generates the update URL for DuckDNS with the specified parameters.
func (ddc *Client) UpdateURL(params Params) (string, error) {
	if params.Domains == "" {
		return "", fmt.Errorf("domain cannot be empty")
	}

	url := fmt.Sprintf(
		"%s/update?domains=%s&token=%s&verbose=%t",
		ddc.host,
		params.Domains,
		params.Token,
		ddc.verbose,
	)

	if params.Clear {
		url += "&clear=true"
	}

	if params.IPv4 != "" {
		url += fmt.Sprintf("&ipv4=%s", params.IPv4)
	}

	if params.IPv6 != "" {
		url += fmt.Sprintf("&ipv6=%s", params.IPv6)
	}

	if params.Txt != "" {
		url += fmt.Sprintf("&txt=%s", params.Txt)
	}

	return url, nil
}
