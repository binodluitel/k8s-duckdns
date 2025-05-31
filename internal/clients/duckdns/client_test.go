package duckdns

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDuckDNSClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DuckDNS Client Suite")
}

var _ = Describe("Client", func() {
	var client *Client
	var err error

	BeforeEach(func() {
		client, err = NewClient("https", "duckdns.org")
		Expect(err).To(Not(HaveOccurred()))
	})

	Describe("UpdateURL", func() {
		It("generates a basic update URL", func() {
			url, err := client.UpdateURL(Params{
				Domains: "testdomain",
				Token:   "mytoken",
			})
			Expect(err).To(Not(HaveOccurred()))
			Expect(url).To(Equal("https://duckdns.org/update?domains=testdomain&token=mytoken&verbose=false"))
		})

		It("includes clear and ipv4 parameters", func() {
			url, err := client.UpdateURL(Params{
				Domains: "testdomain",
				Token:   "mytoken",
				Clear:   true,
				IPv4:    "1.2.3.4",
			})
			Expect(err).To(Not(HaveOccurred()))
			Expect(url).To(Equal("https://duckdns.org/update?domains=testdomain&token=mytoken&verbose=false&clear=true&ipv4=1.2.3.4"))
		})

		It("includes ipv6 and txt parameters", func() {
			url, err := client.UpdateURL(Params{
				Domains: "testdomain",
				Token:   "mytoken",
				IPv6:    "abcd::1",
				Txt:     "sometxt",
			})
			Expect(err).To(Not(HaveOccurred()))
			Expect(url).To(Equal("https://duckdns.org/update?domains=testdomain&token=mytoken&verbose=false&ipv6=abcd::1&txt=sometxt"))
		})

		It("returns error if domain is missing", func() {
			_, err := client.UpdateURL(Params{Token: "mytoken"})
			Expect(err).To(MatchError(fmt.Errorf("domain cannot be empty")))
		})
	})
})
