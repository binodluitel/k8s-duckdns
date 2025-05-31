package config

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = Describe("Config", func() {
	var origEnv map[string]string

	// Helper to save and restore env vars
	saveEnv := func(keys ...string) map[string]string {
		saved := make(map[string]string)
		for _, k := range keys {
			// Check if the environment variable exists.
			// If it doesn't exist, we skip saving it to avoid
			// cluttering the saved map with empty values.
			if _, exists := os.LookupEnv(k); !exists {
				continue
			}
			saved[k] = os.Getenv(k)
		}
		return saved
	}
	restoreEnv := func(saved map[string]string) {
		for k, v := range saved {
			Expect(os.Setenv(k, v)).To(Succeed(), "failed to restore env var %s", k)
		}
	}

	// Helper to reset singleton
	resetConfig := func() {
		mutex.Lock()
		defer mutex.Unlock()
		c = nil
	}

	BeforeEach(func() {
		origEnv = saveEnv("DUCKDNS_PROTOCOL", "DUCKDNS_DOMAIN", "DUCKDNS_VERBOSE")
		resetConfig()
	})

	AfterEach(func() {
		restoreEnv(origEnv)
		resetConfig()
	})

	It("loads default values", func() {
		cfg, err := Get()
		Expect(err).To(Not(HaveOccurred()))
		Expect(cfg.DuckDNS.Protocol).To(Equal("https"))
		Expect(cfg.DuckDNS.Domain).To(Equal("www.duckdns.org"))
		Expect(cfg.DuckDNS.Verbose).To(BeTrue())
	})

	It("overrides values from environment variables", func() {
		Expect(os.Setenv("DUCKDNS_PROTOCOL", "http")).To(Succeed())
		Expect(os.Setenv("DUCKDNS_DOMAIN", "custom.duckdns.org")).To(Succeed())
		Expect(os.Setenv("DUCKDNS_VERBOSE", "false")).To(Succeed())
		resetConfig()
		cfg, err := Get()
		Expect(err).To(Not(HaveOccurred()))
		Expect(cfg.DuckDNS.Protocol).To(Equal("http"))
		Expect(cfg.DuckDNS.Domain).To(Equal("custom.duckdns.org"))
		Expect(cfg.DuckDNS.Verbose).To(BeFalse())
	})

	It("returns the same instance (singleton)", func() {
		cfg1, err1 := Get()
		cfg2, err2 := Get()
		Expect(err1).To(Not(HaveOccurred()))
		Expect(err2).To(Not(HaveOccurred()))
		Expect(cfg1).To(BeIdenticalTo(cfg2))
	})

	It("errors on invalid environment variable values", func() {
		Expect(os.Setenv("DUCKDNS_VERBOSE", "not_bool")).To(Succeed())
		resetConfig()
		_, err := Get()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("DUCKDNS_VERBOSE"))
	})

	It("panics on MustGet if envconfig fails", func() {
		Expect(os.Setenv("DUCKDNS_VERBOSE", "not_bool")).To(Succeed())
		resetConfig()
		Expect(func() { MustGet() }).To(Panic())
	})

	It("returns config without panic", func() {
		Expect(os.Setenv("DUCKDNS_PROTOCOL", "https")).To(Succeed())
		Expect(os.Setenv("DUCKDNS_DOMAIN", "duckdns.org")).To(Succeed())
		Expect(os.Setenv("DUCKDNS_VERBOSE", "true")).To(Succeed())
		resetConfig()
		cfg := MustGet()
		Expect(cfg.DuckDNS.Protocol).To(Equal("https"))
		Expect(cfg.DuckDNS.Domain).To(Equal("duckdns.org"))
		Expect(cfg.DuckDNS.Verbose).To(BeTrue())
	})
})
