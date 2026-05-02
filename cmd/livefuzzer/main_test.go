package main

import (
	"testing"

	"github.com/MariusVanDerWijden/tx-fuzz/flags"
)

func TestInitAppRegistersCampaignAndReplayCommands(t *testing.T) {
	app := initApp()
	seenCampaign := false
	seenReplay := false
	for _, cmd := range app.Commands {

		if cmd.Name == "airdrop" {
			foundCount := false
			for _, flag := range cmd.Flags {
				if flag.Names()[0] == flags.CountFlag.Name {
					foundCount = true
				}
			}
			if !foundCount {
				t.Fatalf("airdrop command missing %s flag", flags.CountFlag.Name)
			}
		}
		if cmd.Name == "campaign" {
			seenCampaign = true
		}
		if cmd.Name == "replay" {
			seenReplay = true
			foundBundle := false
			foundEndpoints := false
			for _, flag := range cmd.Flags {
				switch flag.Names()[0] {
				case flags.BundleFlag.Name:
					foundBundle = true
				case flags.EndpointsFlag.Name:
					foundEndpoints = true
				}
			}
			if !foundBundle || !foundEndpoints {
				t.Fatalf("replay flags missing bundle/endpoints: bundle=%v endpoints=%v", foundBundle, foundEndpoints)
			}
		}
	}
	if !seenCampaign || !seenReplay {
		t.Fatalf("expected campaign and replay commands, got campaign=%v replay=%v", seenCampaign, seenReplay)
	}
}
