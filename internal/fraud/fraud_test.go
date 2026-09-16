package fraud

import "testing"

func TestIsOfficial(t *testing.T) {
	c := New(nil, []string{"pmsvanidhi.mohua.gov.in"})

	official := []string{
		"pmkisan.gov.in",
		"pmsvanidhi.mohua.gov.in",
		"nhfdc.nic.in",
		"mudra.org.in",
		"retail.onlinesbi.sbi", // subdomain of an allowlisted institution
		"www.kviconline.gov.in",
	}
	for _, host := range official {
		if !c.IsOfficial(host) {
			t.Errorf("%s should be treated as official", host)
		}
	}

	// The cases that matter: domains built to look official. A lookalike must
	// never inherit trust from embedding a government string in its name.
	impostors := []string{
		"pmkisan-gov.in",
		"pmkisan.gov.in.apply-online.com",
		"sarkari-yojana-apply.com",
		"pmsvanidhi-registration.in",
		"mudra-loan-apply.co",
		"gov-in-schemes.net",
		"",
	}
	for _, host := range impostors {
		if c.IsOfficial(host) {
			t.Errorf("%s must NOT be treated as official", host)
		}
	}
}
