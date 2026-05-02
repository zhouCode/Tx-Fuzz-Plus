package flags

import "testing"

func TestCampaignFlagsIncludeSharedConfigFlags(t *testing.T) {
	required := map[string]bool{
		SkFlag.Name:        false,
		NoALFlag.Name:      false,
		CountFlag.Name:     false,
		TxCountFlag.Name:   false,
		GasLimitFlag.Name:  false,
		EndpointsFlag.Name: false,
		ELClientFlag.Name:  false,
	}
	for _, flag := range CampaignFlags {
		for name := range required {
			if flag.Names()[0] == name {
				required[name] = true
			}
		}
	}
	for name, seen := range required {
		if !seen {
			t.Fatalf("campaign flags missing %q", name)
		}
	}
}
